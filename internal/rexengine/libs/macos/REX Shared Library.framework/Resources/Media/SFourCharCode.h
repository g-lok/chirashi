#pragma once

#include <cstdint>

namespace RSMedia {

class CMedia;


#ifndef FOUR_CHARACTER_CODE
#define FOUR_CHARACTER_CODE(code) static_cast<RSMedia::TFourCharacterCode>((static_cast<std::uint32_t>(code[0]) << 24) | (static_cast<std::uint32_t>(code[1]) << 16) | (static_cast<std::uint32_t>(code[2]) << 8) | static_cast<std::uint32_t>(code[3]))
#endif // ndef FOUR_CHARACTER_CODE

#ifndef CONST_FOUR_CHARACTER_CODE
#define CONST_FOUR_CHARACTER_CODE(code) static_cast<RSMedia::TFourCharacterCode>(code)
#endif // ndef CONST_FOUR_CHARACTER_CODE


/////////////////////		TFourCharacterCode

typedef std::uint32_t TFourCharacterCode;


// ### FL: Unused
TFourCharacterCode ReadFourCharCode(const CMedia& iMedia);
void WriteFourCharCode(CMedia& iMedia, const TFourCharacterCode& iCode);


}	//	RSMedia


