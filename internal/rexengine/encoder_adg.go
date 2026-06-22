package rexengine

import (
	"bytes"
	"fmt"
	"io"
)

func EncodeDrumRackADG(w io.Writer, slices []SliceExtraction, samplePaths []string, baseNote int) error {
	if len(slices) == 0 {
		return fmt.Errorf("cannot encode ADG: no slices")
	}
	if len(samplePaths) != len(slices) {
		return fmt.Errorf("cannot encode ADG: %d slices but %d paths", len(slices), len(samplePaths))
	}

	xml := buildDrumRackXML(slices, samplePaths, baseNote)
	return writeGZip(w, xml)
}

func buildDrumRackXML(slices []SliceExtraction, samplePaths []string, baseNote int) string {
	var buf bytes.Buffer

	buf.WriteString(`<Ableton MajorVersion="5">
  <GroupDevicePreset>
    <Device>
      <DrumGroupDevice>
        <DrumPadsListWrapper>
          <Pads>`)

	for i := range slices {
		note := baseNote + i
		if note > 127 {
			note = 127
		}
		buf.WriteString(fmt.Sprintf(`
            <Pad>
              <MidiNote>
                <Value>%d</Value>
              </MidiNote>
              <Chain>
                <DeviceChain>
                  <Devices>
                    <OriginalSimpler>
                      <SampleRef>
                        <FileRef>
                          <Path Value="%s"/>
                        </FileRef>
                      </SampleRef>
                    </OriginalSimpler>
                  </Devices>
                </DeviceChain>
              </Chain>
            </Pad>`, note, escapeXML(samplePaths[i])))
	}

	buf.WriteString(`
          </Pads>
        </DrumPadsListWrapper>
      </DrumGroupDevice>
    </Device>
    <BranchPresets>`)

	for i := range slices {
		note := baseNote + i
		if note > 127 {
			note = 127
		}
		buf.WriteString(fmt.Sprintf(`
      <DrumBranchPreset>
        <DevicePresets>
          <AbletonDevicePreset>
            <Device>
              <DrumCell>
                <UserSample>
                  <Value>
                    <SampleRef>
                      <FileRef>
                        <Path Value="%s"/>
                      </FileRef>
                    </SampleRef>
                  </Value>
                </UserSample>
              </DrumCell>
            </Device>
          </AbletonDevicePreset>
        </DevicePresets>
        <ZoneSettings ReceivingNote="%d"/>
      </DrumBranchPreset>`, escapeXML(samplePaths[i]), note))
	}

	buf.WriteString(`
    </BranchPresets>
  </GroupDevicePreset>
</Ableton>`)

	return buf.String()
}


