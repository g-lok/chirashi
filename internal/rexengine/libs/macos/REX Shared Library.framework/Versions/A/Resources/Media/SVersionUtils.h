#pragma once


#include "SVersion.h"
#include "Core/Text/TextPackage.h"

namespace RSFile {
class VItem;
}

namespace RSMedia {

// JP: Not suitable for formatting version number that are presented to the user
// JP: For user-visible version numbers, use NSGUILibUtils::FormatVersionAsString() in App/GUILibSupport.h instead.
RSText::TString FormatVersionAsStringUS(
	const CVersion& iVersion,
	const RSText::TString& iEmptyVersionString = RSText::TString());
CVersion ParseVersionStringUS(const RSText::TString& iVersionString);


//	This reads the version resource from the file, if any version resource exists.
#if WINDOWS
CVersion GetFileVersion(const RSFile::VItem& iFileItem);
#endif // WINDOWS

#if MAC
CVersion GetFileVersion(CFBundleRef iBundle);
CVersion GetFileVersion(const RSFile::VItem& iDir);
#endif // MAC



enum EFileBuildFlavor {
	kFileBuildFlavor_Debugging = 55,
	kFileBuildFlavor_Testing,
	kFileBuildFlavor_Deployment,
	kFileBuildFlavor_Unknown
};

#if WINDOWS
EFileBuildFlavor GetFileBuildFlavor(const RSFile::VItem& iFileItem);
#endif // WINDOWS

#if MAC
EFileBuildFlavor GetFileBuildFlavor(CFBundleRef iBundle);
EFileBuildFlavor GetFileBuildFlavor(const RSFile::VItem& iDirItem);
#endif // MAC


}	//	RSMedia

