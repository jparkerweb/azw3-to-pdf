package ebook

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
)

// Limits on what an archive may unpack to. A book is read whole into memory,
// so these are what stands between a hostile file and an exhausted machine.
// Sizes are counted from the bytes actually read, never from the headers,
// which an archive can set to anything. The two size caps are variables only
// so tests can lower them.
var (
	maxEntryBytes int64 = 256 << 20
	maxTotalBytes int64 = 1 << 30
)

const maxRatio = 200

// errBrokenArchive reports a zip file that cannot be read at all.
var errBrokenArchive = errors.New("the archive is damaged and cannot be read")

// zipArchive is the only way readers touch zip files (EPUB, CBZ, .fb2.zip).
// Entries are looked up by a cleaned name and read into memory; nothing is
// ever extracted to disk.
type zipArchive struct {
	files map[string]*zip.File
	lower map[string]*zip.File
	order []string
	total int64
}

// openZip opens an archive held in memory.
func openZip(data []byte) (*zipArchive, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errBrokenArchive, err)
	}
	return newZipArchive(r.File), nil
}

// newZipArchive indexes the files of an already opened archive. An entry
// whose name is absolute or climbs out of the archive is dropped: a
// legitimate book never needs one, and dropping it means no later path
// arithmetic can reach it.
func newZipArchive(files []*zip.File) *zipArchive {
	a := &zipArchive{files: map[string]*zip.File{}, lower: map[string]*zip.File{}}
	for _, f := range files {
		raw := strings.ReplaceAll(f.Name, `\`, "/")
		if f.FileInfo().IsDir() || strings.HasSuffix(raw, "/") || isAbsoluteName(raw) {
			continue
		}
		name := cleanZipName(raw)
		if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "../") {
			continue
		}
		if _, dup := a.files[name]; dup {
			continue
		}
		a.files[name] = f
		a.order = append(a.order, name)
		if _, dup := a.lower[strings.ToLower(name)]; !dup {
			a.lower[strings.ToLower(name)] = f
		}
	}
	return a
}

func isAbsoluteName(name string) bool {
	return strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':')
}

// cleanZipName normalises a name for lookup: forward slashes, no dot
// segments and no leading slash.
func cleanZipName(name string) string {
	name = path.Clean(strings.ReplaceAll(name, `\`, "/"))
	return strings.TrimPrefix(name, "/")
}

// names lists the archive's files in archive order.
func (a *zipArchive) names() []string {
	return a.order
}

func (a *zipArchive) has(name string) bool {
	return a.lookup(name) != nil
}

// lookup finds an entry by name, falling back to a case-insensitive match
// because books built on Windows and macOS often disagree with their own
// manifests about case.
func (a *zipArchive) lookup(name string) *zip.File {
	if strings.HasPrefix(name, "../") || name == ".." {
		return nil
	}
	name = cleanZipName(name)
	if strings.HasPrefix(name, "../") || name == ".." {
		return nil
	}
	if f, ok := a.files[name]; ok {
		return f
	}
	return a.lower[strings.ToLower(name)]
}

// read returns an entry's contents, enforcing the size limits.
func (a *zipArchive) read(name string) ([]byte, error) {
	f := a.lookup(name)
	if f == nil {
		return nil, fmt.Errorf("%s is missing from the archive", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %v", name, errBrokenArchive, err)
	}
	defer func() { _ = rc.Close() }()

	// Stop reading as soon as either cap is passed, so a zip bomb costs at
	// most a few hundred times its packed size in memory rather than the
	// whole entry cap. Entries up to 1 MiB are exempt from the ratio, since
	// small runs of markup compress very well.
	limit, bomb := maxEntryBytes, false
	if r := max(1<<20, maxRatio*int64(min(f.CompressedSize64, uint64(maxEntryBytes)))); r < limit {
		limit, bomb = r, true
	}
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %v", name, errBrokenArchive, err)
	}
	if int64(len(data)) > limit {
		if bomb {
			return nil, fmt.Errorf("%s looks like a zip bomb: it unpacks to more than %d times its packed size", name, maxRatio)
		}
		return nil, fmt.Errorf("%s is larger than %s once unpacked", name, sizeText(maxEntryBytes))
	}
	a.total += int64(len(data))
	if a.total > maxTotalBytes {
		return nil, fmt.Errorf("%s: the book is larger than %s once unpacked", name, sizeText(maxTotalBytes))
	}
	return data, nil
}

// sizeText writes a size cap the way people say it: "1 GiB", "256 MiB".
func sizeText(n int64) string {
	if n >= 1<<30 && n%(1<<30) == 0 {
		return fmt.Sprintf("%d GiB", n>>30)
	}
	return fmt.Sprintf("%d MiB", n>>20)
}

// resolve turns an href found in the entry base into an archive name. The
// href is URL-escaped and may carry a fragment or query, neither of which
// names a file.
func (a *zipArchive) resolve(base, href string) string {
	if i := strings.IndexAny(href, "#?"); i >= 0 {
		href = href[:i]
	}
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	href = strings.ReplaceAll(href, `\`, "/")
	if strings.HasPrefix(href, "/") {
		return cleanZipName(href)
	}
	return cleanZipName(path.Join(path.Dir(strings.ReplaceAll(base, `\`, "/")), href))
}
