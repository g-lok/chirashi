#include "StdInclude.h"

#include "SVersionUtils.h"

#if MAC

#include "Core/Debug/CheckedCast.h"
#include "Core/Text/TextPackage.h"
#include "Core/File/RSFile.h"

#include <CoreFoundation/CoreFoundation.h>

#include <cstring>


namespace RSMedia {


static RSOS::CCFHandle<CFDictionaryRef> GetBundleInfoDictionary(const RSFile::VItem& iDirItem) {
	auto bundleURLHandle = iDirItem.GetCFURL();
	ASSERT(bundleURLHandle.Get() != NULL);
	RSOS::CCFHandle<CFDictionaryRef> rDictionaryHandle(CFBundleCopyInfoDictionaryInDirectory(bundleURLHandle.Get()), true);
	return rDictionaryHandle;
}

static RSOS::CCFHandle<CFDictionaryRef> GetBundleInfoDictionary(CFBundleRef iBundle) {
	RSOS::CCFHandle<CFDictionaryRef> rDictionaryHandle(CFBundleGetInfoDictionary(iBundle), false);
	return rDictionaryHandle;
}




static CVersion GetVersionFromInfoDictionary(CFDictionaryRef iInfoDictionary) {
	std::uint32_t buildNumber = 0;
	{
		CFTypeRef buildNumberTypeRef = NULL;
		Boolean buildNumberPresent = CFDictionaryGetValueIfPresent(iInfoDictionary, CFSTR("PropellerheadBuildNumber"), &buildNumberTypeRef); // ??? JZ: Replace with Reason Studios
		if (buildNumberPresent) {
			CFTypeID buildNumberType = ::CFGetTypeID(buildNumberTypeRef);
			if (buildNumberType == CFStringGetTypeID()) {
				RSText::TString buildNumberAsString = RSText::FromCFString(reinterpret_cast<CFStringRef>(buildNumberTypeRef));
				if ((buildNumberAsString.length() > 0) && (buildNumberAsString != RSText::FromASCII("n/a"))) {
					std::int64_t parsedNumber = 0;
					IF_DEBUG(bool good =) RSText::StringToInt(buildNumberAsString, parsedNumber);
					ASSERT(good);
					buildNumber = CheckedCast<std::uint32_t>(parsedNumber);
				}
			}
			else if (buildNumberType == CFNumberGetTypeID()) {
				IF_DEBUG(Boolean succesful =) CFNumberGetValue(reinterpret_cast<CFNumberRef>(buildNumberTypeRef), kCFNumberSInt32Type, &buildNumber);
				ASSERT(succesful);
			}
		}
	}

	std::uint32_t major = 0;
	std::uint32_t minor = 0;
	std::uint32_t revision = 0;
	CVersion::EStage stage = CVersion::kRelease;
	std::uint32_t stageNumber = 0;
	{
		CFTypeRef bundleVersionTypeRef = NULL;
		Boolean versionNumberPresent = CFDictionaryGetValueIfPresent(iInfoDictionary, kCFBundleVersionKey, &bundleVersionTypeRef);
		if (versionNumberPresent) {
			CFTypeID bundleVersionNumberType = CFGetTypeID(bundleVersionTypeRef);
			if (bundleVersionNumberType == CFStringGetTypeID()) {
				RSText::TString bundleVersionNumberAsString = RSText::FromCFString(reinterpret_cast<CFStringRef>(bundleVersionTypeRef));
				CVersion parsed = ParseVersionStringUS(bundleVersionNumberAsString);
				major=parsed.GetMajor();
				minor=parsed.GetMinor();
				revision=parsed.GetRevision();
				stage=parsed.GetStage();
				stageNumber=parsed.GetStageNumber();
			}
			else if (bundleVersionNumberType == ::CFNumberGetTypeID()) {
				IF_DEBUG(Boolean succesful =) CFNumberGetValue(reinterpret_cast<CFNumberRef>(bundleVersionTypeRef), kCFNumberSInt32Type, &major);
				ASSERT(succesful);
			}
		}
	}

	return CVersion(major, minor, revision, stage, stageNumber, buildNumber);
}

CVersion GetFileVersion(CFBundleRef iBundle) {
	ASSERT(iBundle != NULL);
	CVersion rVersion(0, 0);

	RSOS::CCFHandle<CFDictionaryRef> dictionary = GetBundleInfoDictionary(iBundle);
	if (dictionary.Get() != NULL) {
		rVersion = GetVersionFromInfoDictionary(dictionary.Get());
	}

	return rVersion;
}

CVersion GetFileVersion(const RSFile::VItem& iDirItem) {
	CVersion rVersion(0, 0);

	RSOS::CCFHandle<CFDictionaryRef> dictionary = GetBundleInfoDictionary(iDirItem);
	if (dictionary.Get() != NULL) {
		rVersion = GetVersionFromInfoDictionary(dictionary.Get());
	}

	return rVersion;
}







static EFileBuildFlavor GetBuildFlavorFromInfoDictionary(CFDictionaryRef iInfoDictionary) {
	EFileBuildFlavor rBuildFlavor = kFileBuildFlavor_Unknown;

	RSText::TString buildFlavorString;
	{
		CFTypeRef buildFlavorTypeRef = NULL;
		Boolean buildFlavorPresent = CFDictionaryGetValueIfPresent(iInfoDictionary, CFSTR("PropellerheadBuildFlavor"), &buildFlavorTypeRef); // ??? JZ: Replace with Reason Studios
		if (buildFlavorPresent) {
			CFTypeID buildFlavorType = CFGetTypeID(buildFlavorTypeRef);
			if (buildFlavorType == CFStringGetTypeID()) {
				buildFlavorString = RSText::FromCFString(reinterpret_cast<CFStringRef>(buildFlavorTypeRef));
			}
			else {
				// JP: Weird property list?
				ASSERT(false);
			}
		}
	}

	if (RSText::EqualsIgnoreCase(buildFlavorString, RSText::FromASCII("Debugging"))) {
		rBuildFlavor = kFileBuildFlavor_Debugging;
	}
	else if (RSText::EqualsIgnoreCase(buildFlavorString, RSText::FromASCII("Testing"))) {
		rBuildFlavor = kFileBuildFlavor_Testing;
	}
	else if (RSText::EqualsIgnoreCase(buildFlavorString, RSText::FromASCII("Deployment"))) {
		rBuildFlavor = kFileBuildFlavor_Deployment;
	}
	else {
		rBuildFlavor = kFileBuildFlavor_Unknown;
	}

	return rBuildFlavor;
}


EFileBuildFlavor GetFileBuildFlavor(CFBundleRef iBundle) {
	ASSERT(iBundle != NULL);
	EFileBuildFlavor rBuildFlavor = kFileBuildFlavor_Unknown;

	RSOS::CCFHandle<CFDictionaryRef> dictionary = GetBundleInfoDictionary(iBundle);
	if (dictionary.Get() != NULL) {
		rBuildFlavor = GetBuildFlavorFromInfoDictionary(dictionary.Get());
	}

	return rBuildFlavor;
}


EFileBuildFlavor GetFileBuildFlavor(const RSFile::VItem& iDirItem) {
	EFileBuildFlavor rBuildFlavor = kFileBuildFlavor_Unknown;

	RSOS::CCFHandle<CFDictionaryRef> dictionary = GetBundleInfoDictionary(iDirItem);
	if (dictionary.Get() != NULL) {
		rBuildFlavor = GetBuildFlavorFromInfoDictionary(dictionary.Get());
	}

	return rBuildFlavor;
}


}	//	RSMedia


#endif	//	MAC

