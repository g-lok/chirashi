#include "StdInclude.h"

#include "SVersionUtils.h"

#include <cstdlib>

namespace RSMedia {

namespace {

std::uint32_t ExtractNextNumber(RSText::TString& inOutString) {
	RSText::TString::size_type periodPosition = inOutString.find('.');
	RSText::TString numberString;
	if (periodPosition != RSText::TString::npos) {
		numberString = inOutString.substr(0, periodPosition);
		inOutString = inOutString.substr(periodPosition + 1);
	}
	else {
		numberString = inOutString;
		inOutString.clear();
	}
	// FL: atoi returns 0 if empty string or no numbers
	std::uint32_t parsedNumber = std::atoi(RSText::ToUTF8(numberString).c_str());
	return parsedNumber;
}

} // anon


RSText::TString FormatVersionAsStringUS(const CVersion& iVersion, const RSText::TString& iEmptyVersionString) {
	RSText::TString result;
	if (iVersion.IsNull() ) {
		result = iEmptyVersionString;
	}
	else {
		if (iVersion.GetRevision() > 0) {
			std::vector<RSText::TString> variables;
			variables.push_back(RSText::IntToStringUS(iVersion.GetMajor()));
			variables.push_back(RSText::IntToStringUS(iVersion.GetMinor()));
			variables.push_back(RSText::IntToStringUS(iVersion.GetRevision()));
			result = RSText::ExpandStringWithVariables(RSText::FromASCII("^0.^1.^2"), 0, variables);
		}
		else {
			std::vector<RSText::TString> variables;
			variables.push_back(RSText::IntToStringUS(iVersion.GetMajor()));
			variables.push_back(RSText::IntToStringUS(iVersion.GetMinor()));
			result = RSText::ExpandStringWithVariables(RSText::FromASCII("^0.^1"), 0, variables);
		}

		if ((iVersion.GetStage() != CVersion::kRelease) || (iVersion.GetStageNumber() != 0)) {
			RSText::TString stageLetter;
			switch (iVersion.GetStage()) {
				case CVersion::kDevelopment: stageLetter = RSText::FromASCII("d"); break;
				case CVersion::kAlpha: stageLetter = RSText::FromASCII("a"); break;
				case CVersion::kBeta: stageLetter = RSText::FromASCII("b"); break;
				case CVersion::kRelease: stageLetter = RSText::FromASCII("f"); break;
				default: ASSERT(false); stageLetter = RSText::FromASCII("d"); break;
			}
			std::vector<RSText::TString> variables;
			variables.push_back(result);
			variables.push_back(stageLetter);
			variables.push_back(RSText::IntToStringUS(iVersion.GetStageNumber()));
			result = RSText::ExpandStringWithVariables(RSText::FromASCII("^0^1^2"), 0, variables);
		}

		if (iVersion.GetBuildNumber() > 0) {
			std::vector<RSText::TString> variables;
			variables.push_back(result);
			variables.push_back(RSText::IntToStringUS(iVersion.GetBuildNumber()));
			result = RSText::ExpandStringWithVariables(RSText::FromASCII("^0 build ^1"), 0, variables);
		}
	}
	return result;
}

CVersion ParseVersionStringUS(const RSText::TString& iVersionString) {
	std::uint32_t major = 0;
	std::uint32_t minor = 0;
	std::uint32_t revision = 0;
	CVersion::EStage stage = CVersion::kRelease;
	std::uint32_t stageNumber = 0;
	if ((iVersionString.length() > 0) && (iVersionString != RSText::FromASCII("n/a"))) {
		RSText::TString mainPart;
		RSText::TString stagePart;
		RSText::TString::size_type startOfStagePart = iVersionString.find_first_of(RSText::FromASCII("dabf"), 0);
		if (startOfStagePart != RSText::TString::npos) {
			mainPart = iVersionString.substr(0, startOfStagePart);
			stagePart = iVersionString.substr(startOfStagePart);
		}
		else {
			mainPart = iVersionString;
		}
		major = ExtractNextNumber(mainPart);
		minor = ExtractNextNumber(mainPart);
		revision = ExtractNextNumber(mainPart);
		if (stagePart.length() > 0) {
			RSText::TChar stageCharacter = stagePart[0];
			switch (stageCharacter) {
				case 'd':
					stage = CVersion::kDevelopment;
					break;
				case 'a':
					stage = CVersion::kAlpha;
					break;
				case 'b':
					stage = CVersion::kBeta;
					break;
				case 'f':
					stage = CVersion::kRelease;
					break;
				default:
					BTHROW RSBacteria::XFormatViolation("");
			}
			stagePart = stagePart.substr(1);
			stageNumber = std::atoi(RSText::ToUTF8(stagePart).c_str());
		}
	}
	return CVersion(major, minor, revision, stage, stageNumber);
}


} // RSMedia
