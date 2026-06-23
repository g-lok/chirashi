package engine

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

func EncodeSimplerADV(w io.Writer, extraction *SliceExtraction, sampleRelPath string) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode ADV preset: extraction data is empty")
	}
	xml := buildSimplerXML(extraction, sampleRelPath, false)
	return writeGZip(w, xml)
}

func EncodeSimplerALS(w io.Writer, extraction *SliceExtraction, sampleRelPath string) error {
	if extraction == nil || len(extraction.Interleaved) == 0 {
		return fmt.Errorf("cannot encode ALS preset: extraction data is empty")
	}
	xml := buildSimplerXML(extraction, sampleRelPath, true)
	return writeGZip(w, xml)
}

func buildSimplerXML(extraction *SliceExtraction, sampleRelPath string, als bool) string {
	slices := splitExtractionIntoSlices(extraction)
	if len(slices) == 0 {
		slices = []SliceExtraction{*extraction}
	}

	sampleRate := extraction.Metadata.SampleRate
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	channels := extraction.Metadata.Channels
	if channels <= 0 {
		channels = 2
	}

	totalFrames := extraction.TotalFrames
	if totalFrames <= 0 {
		totalFrames = len(extraction.Interleaved) / channels
	}

	var buf bytes.Buffer

	buf.WriteString(`<Ableton MajorVersion="5">`)

	if als {
		buf.WriteString(`
  <LiveSet>
    <Tracks>
      <MidiTrack>
        <Name>
          <EffectiveName Value="Imported Slices"/>
        </Name>
        <DeviceChain>
          <Devices>`)
	}

	buf.WriteString(`
    <OriginalSimpler>
      <SampleRef>
        <FileRef>
          <Path Value="`)
	buf.WriteString(escapeXML(sampleRelPath))
	buf.WriteString(`"/>
          <RelativePath Value="`)
	buf.WriteString(escapeXML(sampleRelPath))
	buf.WriteString(`"/>
        </FileRef>
        <DefaultDuration>
          <Value>`)
	fmt.Fprintf(&buf, "%d", totalFrames)
	buf.WriteString(`</Value>
        </DefaultDuration>
        <DefaultSampleRate>
          <Value>44100</Value>
        </DefaultSampleRate>
      </SampleRef>
      <Player>
        <MultiSampleMap>
          <SampleParts>`)

	frameOffset := 0
	for i, s := range slices {
		label := fmt.Sprintf("Slice %02d", i+1)
		if len(s.CuePoints) > 0 && s.CuePoints[0].Label != "" {
			label = s.CuePoints[0].Label
		}

		startTime := float64(frameOffset) / float64(sampleRate)
		frameOffset += s.TotalFrames
		endTime := float64(frameOffset) / float64(sampleRate)

		partFrames := s.TotalFrames
		if partFrames <= 0 {
			partFrames = len(s.Interleaved) / max(1, channels)
		}

			buf.WriteString(fmt.Sprintf(`
            <MultiSamplePart HasImportedSlicePoints="true">
              <Name>
                <Value>%s</Value>
              </Name>
              <SampleRef>
                <FileRef>
                  <Path Value="%s"/>
                  <RelativePath Value="%s"/>
                </FileRef>
                <DefaultDuration>
                  <Value>%d</Value>
                </DefaultDuration>
                <DefaultSampleRate>
                  <Value>%d</Value>
                </DefaultSampleRate>
              </SampleRef>
              <SlicingBeatGrid>
                <Value>4</Value>
              </SlicingBeatGrid>
              <SlicingRegions>
                <Value>2</Value>
              </SlicingRegions>
              <InitialSlicePointsFromOnsets>
                <SlicePoint TimeInSeconds="%.6f" Rank="0"/>
                <SlicePoint TimeInSeconds="%.6f" Rank="0"/>
              </InitialSlicePointsFromOnsets>
            </MultiSamplePart>`,
				escapeXML(label),
				escapeXML(sampleRelPath),
				escapeXML(sampleRelPath),
				partFrames,
				sampleRate,
				startTime,
				endTime))
	}

	buf.WriteString(`
          </SampleParts>
        </MultiSampleMap>
      </Player>
    </OriginalSimpler>`)

	if als {
		buf.WriteString(`
        </Devices>
          </DeviceChain>
        </MidiTrack>
      </Tracks>
    </LiveSet>`)
	}

	buf.WriteString(`
</Ableton>`)

	return buf.String()
}

func writeGZip(w io.Writer, xml string) error {
	gw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := gw.Write([]byte(xml)); err != nil {
		return err
	}
	return gw.Close()
}
