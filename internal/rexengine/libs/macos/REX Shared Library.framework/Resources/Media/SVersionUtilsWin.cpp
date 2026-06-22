#include "StdInclude.h"

#include "SVersionUtils.h"

#if WINDOWS

#include "SOwnedMemoryMedia.h"
#include "Core/File/RSFile.h"

#include <cstring>
#include <cstdint>
#include <limits>





namespace RSMedia {

// ??? FL: Unused?
#if 0
namespace RSInternal {

	const RSText::TString::size_type kFileNameSize=260;
	const RSText::TString::size_type kFileSystemNameSize=10;
	const RSText::TString::size_type kFileExtensionSize=8;

} // RSInternal
#endif // 0

static RSText::TString ReadStringValueFromStringResource(std::uint8_t* buffer, const RSText::TString& stringName){
	struct LANGANDCODEPAGE {
		WORD wLanguage;
		WORD wCodePage;
	} *lpTranslate;

	// Read the list of languages and code pages.

	UINT cbTranslate = 0;
	const auto succ = VerQueryValue(reinterpret_cast<LPVOID>(buffer),
		TEXT("\\VarFileInfo\\Translation"),
		(LPVOID*)&lpTranslate,
		&cbTranslate);
	if (FALSE != succ) {

		// Read the file description for each language and code page.

		for (UINT i=0; i < (cbTranslate / sizeof(struct LANGANDCODEPAGE) ); i++) {

			WCHAR SubBlock[511+1];
			wsprintf(SubBlock, L"\\StringFileInfo\\%04x%04x\\%s", lpTranslate[i].wLanguage, lpTranslate[i].wCodePage, stringName.c_str());
			
			WCHAR* lpBuffer = 0;
			UINT dwBytes = 0;
			if (VerQueryValue(buffer, SubBlock, reinterpret_cast<VOID FAR* FAR*>(&lpBuffer), &dwBytes) ) {
				if (dwBytes > 0) {
					std::wstring asWstring(lpBuffer, dwBytes);
					auto rString = RSText::FromWCharPtr(asWstring.c_str() );
					return rString;
				}
			}
		}
	}
	return RSText::TString();
}

static bool StageVersionNumberToStageAndStageNumber(std::uint16_t stageVersionNumber, CVersion::EStage& stage, std::uint32_t& stageNumber){

	// First test for new version format (13-bit stageNumber) 
	std::uint16_t stageTestNumber = (stageVersionNumber & 0xE000);
	switch(stageTestNumber){
		case 0x2000:
			stage = CVersion::kDevelopment;
			stageNumber = stageVersionNumber & 0x1FFF;
			return true;
			break;
		case 0x4000:
			stage = CVersion::kAlpha;
			stageNumber = stageVersionNumber & 0x1FFF;
			return true;
			break;
		case 0x6000:
			stage = CVersion::kBeta;
			stageNumber = stageVersionNumber & 0x1FFF;
			return true;
			break;
		case 0x8000:
			// Release candidate!
			stage = CVersion::kRelease;
			stageNumber = stageVersionNumber & 0x1FFF;
			return true;
			break;
		case 0xA000:
			// Golden Master
			stage = CVersion::kRelease;
			stageNumber = 0;
			return true;
			break;

		default:
			// Probably in old format - see below
			break;
	}

	// Test for old version format (8-bit stagenumber)
	std::uint16_t oldStageTestNumber = (stageVersionNumber >> 8);
	switch(oldStageTestNumber){
		case 0x01:
			stage = CVersion::kDevelopment;
			stageNumber = stageVersionNumber & 0xff;
			return true;
			break;
		case 0x02:
			stage = CVersion::kAlpha;
			stageNumber = stageVersionNumber & 0xff;
			return true;
			break;
		case 0x04:
			stage = CVersion::kBeta;
			stageNumber = stageVersionNumber & 0xff;
			return true;
			break;
		case 0x08:
			// Release candidate!
			stage = CVersion::kRelease;
			stageNumber = stageVersionNumber & 0xff;
			return true;
			break;
		case 0x10:
			// Golden Master
			stage = CVersion::kRelease;
			stageNumber = 0;
			return true;
			break;

		default:
			// Bad format
			stage = CVersion::kRelease;
			stageNumber = 0;
			return false;
			break;
	}
}

RSText::TString WinGetFileVersion(const RSFile::VItem& file, std::uint16_t* major, std::uint16_t* minor, std::uint16_t* revision, std::uint16_t* build, CVersion::EStage* stage, std::uint32_t* stageNumber){
	ASSERT(major != 0);
	ASSERT(minor != 0);
	ASSERT(revision != 0);
	ASSERT(build != 0);
	ASSERT(stage != 0);
	ASSERT(stageNumber != 0);

	RSText::TString filePath = file.GetNativePath();
	const WCHAR* winFilePath = RSText::WCharPtr(filePath);

	std::uint8_t* buffer=0;
	UINT lOldErrorMode=::SetErrorMode(SEM_NOOPENFILEERRORBOX);
	try{
		DWORD lpdwHandle=0;
		DWORD lSize=::GetFileVersionInfoSize(winFilePath,&lpdwHandle);
		if(0==lSize){
			DWORD lError=::GetLastError();

			switch(lError){
				case ERROR_FILE_NOT_FOUND:
					BTHROW RSBacteria::XNotFound("") << RSBacteria::GetTargetType_File();
					break;

					// Zero size but still success?
				case ERROR_SUCCESS:
					BTHROW std::exception();
					break;

				case ERROR_BAD_FORMAT:
					BTHROW std::exception();
					break;

				default:
					BTHROW std::exception();
					break;
			}
		}

		buffer=new std::uint8_t[lSize];

		BOOL resultFlag=::GetFileVersionInfo(winFilePath,0,lSize,(LPVOID)buffer);
		if(!resultFlag){
			BTHROW std::exception();
		}

		VS_FIXEDFILEINFO* lFInfo=0;
		UINT lFLength;

		resultFlag= VerQueryValue(
			reinterpret_cast<LPVOID>(buffer),
			L"\\",
			reinterpret_cast<VOID FAR* FAR*>(&lFInfo),
			&lFLength);
		if(!resultFlag){
			BTHROW std::exception();
		}

		*major = HIWORD(lFInfo->dwFileVersionMS);
		*minor = LOWORD(lFInfo->dwFileVersionMS);
		*revision = HIWORD(lFInfo->dwFileVersionLS);
		*build = 0;
		std::uint16_t stageAndStageNumber = LOWORD(lFInfo->dwFileVersionLS);
		*stage = CVersion::kRelease;
		*stageNumber = 0;

		//
		// Look for Smuggler build-number string, if there is any...
		//

		RSText::TString buildNumberString = ReadStringValueFromStringResource(buffer, RSText::FromASCII("Build Number"));
		if(buildNumberString.length() > 0){
			std::int64_t buildNumber = 0;
			if(RSText::StringToIntUS(buildNumberString, buildNumber)){
				if (buildNumber < 0 || buildNumber > std::numeric_limits<std::uint16_t>::max()) {
					BTHROW RSBacteria::XFormatViolation("");
				}
				*build = static_cast<std::uint16_t>(buildNumber);
			}
		}

		if (stageAndStageNumber > 0) {
			bool isValidVersionNumber = StageVersionNumberToStageAndStageNumber(stageAndStageNumber, *stage, *stageNumber);
			if (!isValidVersionNumber) {
				BTHROW RSBacteria::XFormatViolation("");
			}
		}


		//
		// Look for Smuggler product-version string, if there is any...
		//
		RSText::TString productVersionString = ReadStringValueFromStringResource(buffer, RSText::FromASCII("ProductVersion"));
		if(productVersionString.length() == 0){
			productVersionString = ReadStringValueFromStringResource(buffer, RSText::FromASCII("Product Version"));
		}

		delete[] buffer;
		buffer=0;
		::SetErrorMode(lOldErrorMode);
		return productVersionString;
	}
	catch(...){
		::SetErrorMode(lOldErrorMode);
		delete[] buffer;
		buffer=0;
		BRETHROW;
	}
	//lint -e(527) Unreachable
	ASSERT(false);
}

CVersion GetFileVersion(const RSFile::VItem& iFile) {
	RSText::TString productVersion;
	std::uint16_t major = 666;
	std::uint16_t minor = 666;
	std::uint16_t revision = 666;
	std::uint16_t build = 666;
//	std::uint16_t stageVersionNumber = 666;
	CVersion::EStage stage = CVersion::kRelease;
	std::uint32_t stageNumber = 0;

	productVersion = WinGetFileVersion(iFile, &major, &minor, &revision, &build, &stage, &stageNumber);

	return CVersion(major, minor, revision, stage, stageNumber, build);
}









/**
	@brief Get the SpecialBuild property of a file
	@return the SpecialBuild property of a file if VS_FF_SPECIALBUILD is set. Otherwise it returns an empty string.
*/
static RSText::TString WinGetFileSpecialBuildString(const RSFile::VItem& file){

	const auto filePath = file.GetNativePath();
	const WCHAR* winFilePath = RSText::WCharPtr(filePath);

	UINT lOldErrorMode=::SetErrorMode(SEM_NOOPENFILEERRORBOX);
	try{
		DWORD lpdwHandle=0;
		DWORD lSize=::GetFileVersionInfoSize(winFilePath,&lpdwHandle);
		if(0==lSize){
			DWORD lError=::GetLastError();

			switch(lError){
				case ERROR_FILE_NOT_FOUND:
					BTHROW RSBacteria::XNotFound("") << RSBacteria::GetTargetType_File();
					break;

					// Zero size but still success?
				case ERROR_SUCCESS:
					BTHROW std::exception();
					break;

				case ERROR_BAD_FORMAT:
					BTHROW std::exception();
					break;

				default:
					BTHROW std::exception();
					break;
			}
		}

		std::vector<std::uint8_t> buffer(lSize);
		const LPVOID bufferPtr = buffer.data();

		BOOL resultFlag = GetFileVersionInfo(winFilePath, 0, lSize, bufferPtr);
		if(!resultFlag){
			BTHROW std::exception();
		}

		VS_FIXEDFILEINFO* lFInfo=0;
		UINT lFLength;

		resultFlag = VerQueryValue(
			bufferPtr,
			L"\\",
			reinterpret_cast<VOID FAR* FAR*>(&lFInfo),
			&lFLength);
		if(!resultFlag){
			BTHROW std::exception();
		}
		
		RSText::TString rSpecialBuildString;

		bool isSpecialBuild = (lFInfo->dwFileFlags & VS_FF_SPECIALBUILD) > 0;
		if (isSpecialBuild) {
			WCHAR* specialBuildString=0;
			UINT lFSpecialBuildStringLength;
			resultFlag = VerQueryValue(
				bufferPtr,
				L"\\StringFileInfo\\040904b0\\SpecialBuild",
				reinterpret_cast<LPVOID*>(&specialBuildString),
				&lFSpecialBuildStringLength);
			if(!resultFlag){
				BTHROW std::exception();
			}
			rSpecialBuildString = RSText::FromWCharPtr(specialBuildString);
			ASSERT(!rSpecialBuildString.empty());
		}

		::SetErrorMode(lOldErrorMode);
		return rSpecialBuildString;
	}
	catch(...){
		::SetErrorMode(lOldErrorMode);
		BRETHROW;
	}

	ASSERT(false);
}



EFileBuildFlavor GetFileBuildFlavor(const RSFile::VItem& iFileItem) {
	EFileBuildFlavor rBuildFlavor = kFileBuildFlavor_Unknown;
	RSText::TString specialBuildString = WinGetFileSpecialBuildString(iFileItem);
	if (RSText::EqualsIgnoreCase(specialBuildString, RSText::FromASCII("Debugging"))) {
		rBuildFlavor = kFileBuildFlavor_Debugging;
	}
	else if (RSText::EqualsIgnoreCase(specialBuildString, RSText::FromASCII("Testing"))) {
		rBuildFlavor = kFileBuildFlavor_Testing;
	}
	else if (RSText::EqualsIgnoreCase(specialBuildString, RSText::FromASCII("Deployment"))) {
		rBuildFlavor = kFileBuildFlavor_Deployment;
	}
	else {
		rBuildFlavor = kFileBuildFlavor_Unknown;
	}
	return rBuildFlavor;
}


}	//	RSMedia

#endif	//	WINDOWS




