#pragma once

//	MZ: These functions does human-language related things with strings.

#include "Core/Text/TextPackage.h"

#include <string>


namespace RSMedia {

class CMedia;

// JP: The strings are stored as Windows Latin-1 if possible without loss, otherwise utf-8 is used
// JP: On Windows, CR, CR+LF and LF characters are converted to RSText::kParagraphSeparator characters after reading
RSText::TString ReadCrossPlatformString(const CMedia& iMedia);
void WriteCrossPlatformString(CMedia& iMedia, const RSText::TString& iString);


RSText::TString ReadUTF8String(const CMedia& iMedia);
// JP: This function cannot write strings larger than 16 kB in size
void WriteUTF8String(CMedia& iMedia, const RSText::TString& iString);


}	//	RSMedia

