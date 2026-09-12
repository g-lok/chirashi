package engine

import (
	"io"
)

func EncodeTX16W(w io.WriteSeeker, extraction *SliceExtraction) error {
	pcm := extraction.Interleaved
	if extraction.Metadata.Channels > 1 {
		mono := make([]float32, len(pcm)/extraction.Metadata.Channels)
		for i := 0; i < len(mono); i++ {
			mono[i] = pcm[i*extraction.Metadata.Channels]
		}
		pcm = mono
	}

	// Closest TX16W rate
	srCode := byte(1) // 33333
	if extraction.Metadata.SampleRate < 24000 {
		srCode = 3 // 16667
	} else if extraction.Metadata.SampleRate > 41000 {
		srCode = 2 // 50000
	}

	// TX16W max length is 256k samples
	if len(pcm) > 256*1024 {
		pcm = pcm[:256*1024]
	}

	attackLen := len(pcm)
	repeatLen := 0
	if attackLen > 128*1024 {
		repeatLen = attackLen - 128*1024
		attackLen = 128*1024
	}

	header := make([]byte, 32)
	copy(header[:6], "LM8953")
	header[22] = 0xC9 // Non-looped
	header[23] = srCode
	
	// atc_length bit layout
	header[24] = byte(attackLen & 0xFF)
	header[25] = byte((attackLen >> 8) & 0xFF)
	
	// sr magic bits
	srMagic1 := []byte{0, 0x06, 0x10, 0xF6}
	srMagic2 := []byte{0, 0x52, 0x00, 0x52}
	
	header[26] = byte((attackLen >> 16) & 0x01) | srMagic1[srCode]
	
	header[27] = byte(repeatLen & 0xFF)
	header[28] = byte((repeatLen >> 8) & 0xFF)
	header[29] = byte((repeatLen >> 16) & 0x01) | srMagic2[srCode]

	w.Write(header)

	// 12-bit packing: 2 samples -> 3 bytes
	for i := 0; i < len(pcm)-1; i += 2 {
		s1 := int32(pcm[i] * 2047.0)
		s2 := int32(pcm[i+1] * 2047.0)
		
		b1 := byte(s1 & 0xFF)
		b2 := byte(((s1 >> 8) & 0x0F) | ((s2 << 4) & 0xF0))
		b3 := byte((s2 >> 4) & 0xFF)
		
		w.Write([]byte{b1, b2, b3})
	}
	
	// If odd number of samples, pad with zero for the second sample of the last pair
	if len(pcm)%2 != 0 {
		s1 := int32(pcm[len(pcm)-1] * 2047.0)
		b1 := byte(s1 & 0xFF)
		b2 := byte((s1 >> 8) & 0x0F)
		w.Write([]byte{b1, b2, 0x00})
	}

	return nil
}
