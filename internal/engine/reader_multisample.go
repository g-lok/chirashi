package engine

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type MultisampleReader struct{}

func (r *MultisampleReader) SupportedExtensions() []string {
	return []string{".multisample"}
}

func (r *MultisampleReader) Probe(data []byte) (*RexMetadata, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid multisample zip: %w", err)
	}

	var xmlFile *zip.File
	for _, f := range zr.File {
		if strings.EqualFold(filepath.Base(f.Name), "multisample.xml") {
			xmlFile = f
			break
		}
	}

	if xmlFile == nil {
		return nil, fmt.Errorf("multisample.xml not found in archive")
	}

	rc, err := xmlFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	xmlBytes, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var msXML bitwigMultisampleXML
	if err := xml.Unmarshal(xmlBytes, &msXML); err != nil {
		return nil, fmt.Errorf("failed parsing multisample.xml: %w", err)
	}

	return &RexMetadata{
		Channels:   2,
		SampleRate: 44100,
	}, nil
}

func (r *MultisampleReader) Read(data []byte, targetRate int) ([]SliceExtraction, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid multisample zip: %w", err)
	}

	var xmlFile *zip.File
	for _, f := range zr.File {
		if strings.EqualFold(filepath.Base(f.Name), "multisample.xml") {
			xmlFile = f
			break
		}
	}

	if xmlFile == nil {
		return nil, fmt.Errorf("multisample.xml not found in archive")
	}

	rc, err := xmlFile.Open()
	if err != nil {
		return nil, err
	}
	xmlBytes, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, err
	}

	var msXML bitwigMultisampleXML
	if err := xml.Unmarshal(xmlBytes, &msXML); err != nil {
		return nil, fmt.Errorf("failed parsing multisample.xml: %w", err)
	}

	fileMap := make(map[string]*zip.File)
	for _, f := range zr.File {
		fileMap[strings.ToLower(f.Name)] = f
		fileMap[strings.ToLower(strings.TrimPrefix(f.Name, "./"))] = f
	}

	wavReader := &WAVReader{}
	var resultSlices []SliceExtraction

	for _, samp := range msXML.Samples {
		zipFile := fileMap[strings.ToLower(samp.File)]
		if zipFile == nil {
			zipFile = fileMap[strings.ToLower(strings.TrimPrefix(samp.File, "./"))]
		}

		if zipFile == nil {
			continue
		}

		wrc, err := zipFile.Open()
		if err != nil {
			continue
		}
		wavBytes, err := io.ReadAll(wrc)
		wrc.Close()
		if err != nil {
			continue
		}

		subSlices, err := wavReader.Read(wavBytes, targetRate)
		if err != nil || len(subSlices) == 0 {
			continue
		}

		resultSlices = append(resultSlices, subSlices...)
	}

	if len(resultSlices) == 0 {
		return nil, fmt.Errorf("no valid sample audio found in .multisample archive")
	}

	return resultSlices, nil
}

func init() {
	RegisterReader(&MultisampleReader{})
}
