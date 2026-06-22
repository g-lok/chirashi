#include "StdInclude.h"

#include "SGlobalizedString.h"

#include "SBinary.h"
#include "SMedia.h"
#include "SVersion.h"

#include "Core/Debug/CheckedCast.h"
#include "Core/Text/TextPackageUnicodeChars.h"

#include <cstdlib>
#include <limits>


#if DEBUG
#include "SOwnedMemoryMedia.h"
#endif // DEBUG


namespace RSMedia {


const std::uint32_t kCrossPlatformStringMagic = 0xffffffff;

static const RSMedia::CVersion kVersion10(1,0);
static const RSMedia::CVersion& kCurrentVersion(kVersion10);

void WriteCrossPlatformString(CMedia& media, const RSText::TString& unicodeString){
	// DE: Can we use old style string?
	{
		std::string s = RSText::ToWindowsLatin1Lossy(unicodeString, '?');

		std::string winString=s;

		RSText::TString testString = RSText::FromWindowsLatin1(winString);

		if(testString == unicodeString){
			// We can write it old style without loss!
			std::size_t length = winString.length();
			media.Write32Bit(CheckedCast<std::uint32_t>(length));
			if(length > 0){
				media.Write(CheckedCast<RSMedia::TMediaPos>(winString.length()), winString.c_str());
			}
			return;
		}
	}

	// Need to use new stylee!

	media.Write32Bit(kCrossPlatformStringMagic);
	kCurrentVersion.Write(media);
	if(unicodeString.length()>0){
		std::string utf8String=RSText::ToUTF8(unicodeString);
		ASSERT(utf8String.length() > 0);
		media.Write32Bit(CheckedCast<std::uint32_t>(utf8String.length()));
		media.Write(CheckedCast<RSMedia::TMediaPos>(utf8String.length()), utf8String.c_str());
	}
	else {
		media.Write32Bit(0);
	}
}

RSText::TString ReadCrossPlatformString(const CMedia& media){
	const char kStringEnd2 = '\0';

	RSText::TString rString;

	std::uint32_t length = media.Read32Bit();
	if(kCrossPlatformStringMagic == length){
		RSMedia::CVersion version(media);
		if(kCurrentVersion.IsCompatible(version)){
			std::uint32_t utf8Length = media.Read32Bit();
			if(static_cast<TMediaPos>(utf8Length) > (media.GetSize() - media.GetCurrentPosition())){
				BTHROW RSBacteria::XFormatViolation("");
			}
			if(utf8Length > 0){
				std::string buffer(utf8Length, 0);
				media.Read(utf8Length, buffer.data());
				rString = RSText::FromUTF8(buffer);
			}
			else{
			}
		}
		else{
			BTHROW RSBacteria::XUnsupportedFormat("");
		}
	}
	else{
		if(length > (std::string::size_type)(media.GetSize() - media.GetCurrentPosition())){
			BTHROW RSBacteria::XFormatViolation("");
		}

		std::vector<char> buffer(length + 1, kStringEnd2);
		media.Read(length, &buffer[0]);
		std::string winString(&buffer[0]);

		rString = RSText::FromWindowsLatin1(winString);

#if WINDOWS
		{
			RSText::TString::size_type pos = rString.find(RSText::kCarrageReturn);
			while(pos != RSText::TString::npos){
				rString[pos] = RSText::kParagraphSeparator;
				if(rString[pos+1] == RSText::kLineFeed){
					++pos;
					rString.erase(pos,1);
				}
				pos = rString.find(RSText::kCarrageReturn);
			}
			pos = rString.find(RSText::kLineFeed);
			while(pos != RSText::TString::npos){
				rString[pos] = RSText::kParagraphSeparator;
				pos = rString.find(RSText::kLineFeed);
			}
		}
#endif // WINDOWS
	}
	return rString;
}



RSText::TString ReadUTF8String(const RSMedia::CMedia& iMedia) {
	RSText::TString rReadString;
	std::uint32_t stringSize = iMedia.Read32Bit();
	if (stringSize > 16383) {
		BTHROW RSBacteria::XFormatViolation("");
	}
	if (stringSize > 0) {
		std::vector<char> buffer(stringSize + 1,0);
		iMedia.Read(stringSize, &buffer[0]);
		std::string utf8String(&buffer[0]);
		rReadString = RSText::FromUTF8(utf8String);
	}
	return rReadString;
}

void WriteUTF8String(RSMedia::CMedia& iMedia, const RSText::TString& iString) {
	if (iString.length() > 0) {
		std::string utf8String = RSText::ToUTF8(iString);
		iMedia.Write32Bit(CheckedCast<std::uint32_t>(utf8String.length()));
		iMedia.Write(CheckedCast<RSMedia::TMediaPos>(utf8String.length()), utf8String.c_str());
	}
	else {
		iMedia.Write32Bit(0);
	}
}

#if DEBUG
namespace {
QUICKTEST_SINGLETHREAD("GlobalizedString", "CrossPlatformString")
{
	const auto str = u"kate"_RSs2;
	COwnedMemoryMedia media;
	WriteCrossPlatformString(media, str);
	media.SetCurrentPosition(0);
	auto str2 = ReadCrossPlatformString(media);
	TEST_VERIFY(str == str2);
}

QUICKTEST_SINGLETHREAD("GlobalizedString", "CrossPlatformString2")
{
	const auto str = u"𝐊 𝐋 𝐌 𝐍 𝐎 𝐏 𝐐 𝐑 𝐒 𝐓 𝐔 𝐕 𝐖 𝐗 𝐘 𝐙 𝐚 𝐛 𝐜 𝐝 𝐞"_RSs2;
	COwnedMemoryMedia media;
	WriteCrossPlatformString(media, str);
	media.SetCurrentPosition(0);
	auto str2 = ReadCrossPlatformString(media);
	TEST_VERIFY(str == str2);
}
} // namespace
#endif // DEBUG

}	//	RSMedia
