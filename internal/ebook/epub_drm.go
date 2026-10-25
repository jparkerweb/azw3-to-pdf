package ebook

import (
	"errors"
	"fmt"
)

// ErrDRMLocked reports an EPUB whose content is encrypted by a DRM scheme.
// ebook-to-pdf refuses such books rather than trying to decrypt them.
var ErrDRMLocked = errors.New("the book is locked with DRM")

// drmError names the scheme that locks a book, and unwraps to ErrDRMLocked.
type drmError struct{ scheme string }

func (e *drmError) Error() string {
	return fmt.Sprintf("this EPUB is locked with %s DRM, which ebook-to-pdf does not remove; "+
		"convert a DRM-free copy (many stores and publishers sell them)", e.scheme)
}

func (e *drmError) Unwrap() error { return ErrDRMLocked }

// fontObfuscation lists the encryption algorithms that only scramble
// embedded fonts (IDPF and Adobe). They are not DRM, and the renderer never
// reads a book's fonts anyway, so a book using them converts.
var fontObfuscation = map[string]bool{
	"http://www.idpf.org/2008/embedding": true,
	"http://ns.adobe.com/pdf/enc#RC":     true,
}

// checkEPUBDRM refuses a locked book before anything else in it is read.
func checkEPUBDRM(a *zipArchive) error {
	switch {
	case a.has("META-INF/rights.xml"):
		return &drmError{"Adobe ADEPT"}
	case a.has("META-INF/sinf.xml"):
		return &drmError{"Apple FairPlay"}
	case a.has("META-INF/license.lcpl"):
		return &drmError{"Readium LCP"}
	}
	if !a.has("META-INF/encryption.xml") {
		return nil
	}
	raw, err := a.read("META-INF/encryption.xml")
	if err != nil {
		return err
	}
	var enc struct {
		Data []struct {
			Method struct {
				Algorithm string `xml:"Algorithm,attr"`
			} `xml:"EncryptionMethod"`
		} `xml:"EncryptedData"`
	}
	if err := decodeXML(raw, &enc); err != nil {
		// Unreadable, so there is no telling what it encrypts.
		return &drmError{"an unrecognised"}
	}
	for _, d := range enc.Data {
		if !fontObfuscation[d.Method.Algorithm] {
			return &drmError{"an unrecognised"}
		}
	}
	return nil
}
