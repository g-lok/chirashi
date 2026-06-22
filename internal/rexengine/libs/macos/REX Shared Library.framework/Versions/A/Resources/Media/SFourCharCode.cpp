#include "StdInclude.h"

#include "SFourCharCode.h"
#include "Core/Test/TestPackage.h"
#include "SMedia.h"
#include "SBinary.h"



namespace RSMedia {


RSMedia::TFourCharacterCode ReadFourCharCode(const RSMedia::CMedia& iMedia) {
	std::uint8_t data[4];
	iMedia.Read(4, data);
	const RSMedia::TFourCharacterCode result = static_cast<RSMedia::TFourCharacterCode>(RSMedia::Unpack32BitUnsignedBig(data));
	return result;
}

void WriteFourCharCode(RSMedia::CMedia& iMedia, const RSMedia::TFourCharacterCode& iCode) {
	std::uint8_t data[4];
	RSMedia::Pack32BitUnsignedBig(data, iCode);
	iMedia.Write(4, data);
}




namespace {
	
QUICKTEST_SINGLETHREAD("CONST_FOUR_CHARACTER_CODE", "CONST_FOUR_CHARACTER_CODE_TEST") {
	bool ok = true;
	ok |= (CONST_FOUR_CHARACTER_CODE('abcd') == FOUR_CHARACTER_CODE("abcd"));
	ok |= (CONST_FOUR_CHARACTER_CODE('efgh') == FOUR_CHARACTER_CODE("efgh"));
	ok |= (CONST_FOUR_CHARACTER_CODE('ijkl') == FOUR_CHARACTER_CODE("ijkl"));
	ok |= (CONST_FOUR_CHARACTER_CODE('mnop') == FOUR_CHARACTER_CODE("mnop"));
	ok |= (CONST_FOUR_CHARACTER_CODE('qrst') == FOUR_CHARACTER_CODE("qrst"));
	ok |= (CONST_FOUR_CHARACTER_CODE('uvwx') == FOUR_CHARACTER_CODE("uvwx"));
	ok |= (CONST_FOUR_CHARACTER_CODE('yz12') == FOUR_CHARACTER_CODE("yz12"));
	ok |= (CONST_FOUR_CHARACTER_CODE('3456') == FOUR_CHARACTER_CODE("3456"));
	ok |= (CONST_FOUR_CHARACTER_CODE('7890') == FOUR_CHARACTER_CODE("7890"));

	ok |= (CONST_FOUR_CHARACTER_CODE('ABCD') == FOUR_CHARACTER_CODE("ABCD"));
	ok |= (CONST_FOUR_CHARACTER_CODE('EFGH') == FOUR_CHARACTER_CODE("EFGH"));
	ok |= (CONST_FOUR_CHARACTER_CODE('IJKL') == FOUR_CHARACTER_CODE("IJKL"));
	ok |= (CONST_FOUR_CHARACTER_CODE('MNOP') == FOUR_CHARACTER_CODE("MNOP"));
	ok |= (CONST_FOUR_CHARACTER_CODE('QRST') == FOUR_CHARACTER_CODE("QRST"));
	ok |= (CONST_FOUR_CHARACTER_CODE('VWXY') == FOUR_CHARACTER_CODE("VWXY"));
	ok |= (CONST_FOUR_CHARACTER_CODE('Z   ') == FOUR_CHARACTER_CODE("Z   "));

	ok |= (CONST_FOUR_CHARACTER_CODE('_-.,') == FOUR_CHARACTER_CODE("_-.,"));
	ok |= (CONST_FOUR_CHARACTER_CODE('?!  ') == FOUR_CHARACTER_CODE("?!  "));

	TEST_VERIFY(ok == true);

	static const auto NotZero = CONST_FOUR_CHARACTER_CODE('[]{}');
	static_assert(NotZero != 0, "");
	static const auto IsZero = CONST_FOUR_CHARACTER_CODE('\0\0\0\0');
	static_assert(IsZero == 0, "");
}

} // anon


}	//	RSMedia
