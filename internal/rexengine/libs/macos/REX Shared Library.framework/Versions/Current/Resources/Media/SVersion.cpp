#include "StdInclude.h"

#include "SVersion.h"

#include "Core/Media/SBinary.h"
#include "Core/Media/SMedia.h"

#include <limits>



namespace RSMedia {

const std::uint8_t kVersionBinaryMagic = 0xbc;



CVersion::CVersion(const CMedia& media) :
	fMajor(0),
	fMinor(0),
	fRevision(0),
	fStage(kRelease),
	fStageNumber(0),
	fBuildNumber(0)
{
	if ((media.GetSize() - media.GetCurrentPosition()) < kImageV1Size) {
		BTHROW RSBacteria::XFormatViolation("");
	}
	// JP: Can't compare enum members from different enums without a compiler warning with gcc, so cast them
	ASSERT(static_cast<TMediaPos>(kImageV2Size) > static_cast<TMediaPos>(kImageV1Size));

	std::uint8_t binary[kImageV2Size];
	media.Read(kImageV1Size, &binary[0]);
	if ((binary[kImageV1Magic] == kVersionBinaryMagic) &&
		(binary[kImageV1Major] == 0xff) &&
		(binary[kImageV1Minor] == 0xff) &&
		(binary[kImageV1Revision] == 0xff) &&
		(binary[kImageV1Reserved] == 0xff))
	{
		media.Read(kImageV2Size - kImageV1Size, &binary[kImageV1Size]);
	}
	if(!Unpack(&binary[0])){
		BTHROW RSBacteria::XFormatViolation("");
	}
}


std::uint32_t CVersion::GetMajor() const {
	return fMajor;
}

std::uint32_t CVersion::GetMinor() const {
	return fMinor;
}

std::uint32_t CVersion::GetRevision() const {
	return fRevision;
}

CVersion::EStage CVersion::GetStage() const {
	return fStage;
}

std::uint32_t CVersion::GetStageNumber() const {
	return fStageNumber;
}

std::uint32_t CVersion::GetBuildNumber() const {
	return fBuildNumber;
}

CVersion::EComparisonResult CVersion::Compare(const CVersion& other) const {
	EComparisonResult result = kEqual;
	if (other.fMajor < fMajor) {
		result = kOtherIsOlder;
	}
	else if (other.fMajor > fMajor) {
		result = kOtherIsNewer;
	}
	else {
		if (other.fMinor < fMinor) {
			result = kOtherIsOlder;
		}
		else if (other.fMinor > fMinor) {
			result = kOtherIsNewer;
		}
		else {
			if (other.fRevision < fRevision) {
				result = kOtherIsOlder;
			}
			else if (other.fRevision > fRevision) {
				result = kOtherIsNewer;
			}
			else {
				if (static_cast<int>(other.fStage) < static_cast<int>(fStage)) {
					result = kOtherIsOlder;
				}
				else if (static_cast<int>(other.fStage) > static_cast<int>(fStage)) {
					result = kOtherIsNewer;
				}
				else {
					if (fStage == kRelease) {
						// JP: Special case for release, since release stage 0 is newer than release stage 1 (and 2, and 3, and...)
						ASSERT(other.fStage == kRelease);
						if ((other.fStageNumber == 0) && (fStageNumber > 0)) {
							result = kOtherIsNewer;
						}
						else if ((fStageNumber == 0) && (other.fStageNumber > 0)) {
							result = kOtherIsOlder;
						}
						else if (other.fStageNumber < fStageNumber) {
							result = kOtherIsOlder;
						}
						else if (other.fStageNumber > fStageNumber) {
							result = kOtherIsNewer;
						}
						else{
							result = kEqual;
						}
					}
					else {
						if (other.fStageNumber < fStageNumber) {
							result = kOtherIsOlder;
						}
						else if (other.fStageNumber > fStageNumber) {
							result = kOtherIsNewer;
						}
						else {
							result = kEqual;
						}
					}
				}
			}
		}
	}
	return result;
}

bool CVersion::operator<(const CVersion& o) const {
	return Compare(o) == kOtherIsNewer;
}


bool CVersion::IsNull() const {

	return ( (fMajor == 0) &&
			 (fMinor == 0) &&
			 (fRevision == 0) &&
			 (fStage == kRelease) &&
			 (fStageNumber == 0) &&
			 (fBuildNumber == 0) );
}

bool CVersion::IsNewEnough(const CVersion& requiredVersion) const {
	EComparisonResult comparisonResult = Compare(requiredVersion);
	if (comparisonResult == kOtherIsNewer) {
		return false;
	}
	else {
		return true;
	}
}

bool CVersion::IsEqual(const CVersion& requiredVersion) const{
	EComparisonResult comparisonResult = Compare(requiredVersion);
	if (comparisonResult == kEqual) {
		return true;
	}
	else {
		return false;
	}
}

bool CVersion::IsSupported(const CVersion& oldestSupported, const CVersion& current) const {
	EComparisonResult comparisonResult = Compare(oldestSupported);
	if (((comparisonResult == kOtherIsOlder) || (comparisonResult == kEqual)) && (GetMajor() <= current.GetMajor())) {
		return true;
	}
	else {
		return false;
	}
}

bool CVersion::IsCompatible(const CVersion& current) const{
	if (GetMajor() == current.GetMajor()) {
		return true;
	}
	else {
		return false;
	}
}

void CVersion::Pack(std::uint8_t binary[]) const {
	if (NeedsVersion2()) {
		binary[kImageV2Magic] = kVersionBinaryMagic;
		binary[kImageV2DummyMajor] = 0xff;
		binary[kImageV2DummyMinor] = 0xff;
		binary[kImageV2DummyRevision] = 0xff;
		binary[kImageV2DummyReserved] = 0xff;

		RSMedia::Pack32BitUnsignedBig(&binary[kImageV2Major], GetMajor());
		RSMedia::Pack32BitUnsignedBig(&binary[kImageV2Minor], GetMinor());
		RSMedia::Pack32BitUnsignedBig(&binary[kImageV2Revision], GetRevision());
		std::uint32_t encodedStage = static_cast<std::uint32_t>(GetStage());
		RSMedia::Pack32BitUnsignedBig(&binary[kImageV2Stage], encodedStage);
		RSMedia::Pack32BitUnsignedBig(&binary[kImageV2StageNumber], GetStageNumber());
		RSMedia::Pack32BitUnsignedBig(&binary[kImageV2BuildNumber], GetBuildNumber());
	}
	else {
		binary[kImageV2Magic] = kVersionBinaryMagic;
		binary[kImageV2DummyMajor] = static_cast<std::uint8_t>(GetMajor());
		binary[kImageV2DummyMinor] = static_cast<std::uint8_t>(GetMinor());
		binary[kImageV2DummyRevision] = static_cast<std::uint8_t>(GetRevision());
		binary[kImageV2DummyReserved] = 0;
	}
}

bool CVersion::Unpack(const std::uint8_t binary[]) {
	bool valid = false;
	if ((binary[kImageV1Magic] == kVersionBinaryMagic) &&
		(binary[kImageV1Major] == 0xff) &&
		(binary[kImageV1Minor] == 0xff) &&
		(binary[kImageV1Revision] == 0xff) &&
		(binary[kImageV1Reserved] == 0xff))
	{
		fMajor = RSMedia::Unpack32BitUnsignedBig(&binary[kImageV2Major]);
		fMinor = RSMedia::Unpack32BitUnsignedBig(&binary[kImageV2Minor]);
		fRevision = RSMedia::Unpack32BitUnsignedBig(&binary[kImageV2Revision]);
		std::uint32_t encodedStage = RSMedia::Unpack32BitUnsignedBig(&binary[kImageV2Stage]);

		switch (encodedStage) {
			case kDevelopment:
				fStage = kDevelopment;
				break;
			case kAlpha:
				fStage = kAlpha;
				break;
			case kBeta:
				fStage = kBeta;
				break;
			case kRelease:
				fStage = kRelease;
				break;
			default:
				BTHROW RSBacteria::XFormatViolation("");
		}

		fStageNumber = RSMedia::Unpack32BitUnsignedBig(&binary[kImageV2StageNumber]);
		fBuildNumber = RSMedia::Unpack32BitUnsignedBig(&binary[kImageV2BuildNumber]);
		valid = true;
	}
	else if (binary[kImageV1Magic] == kVersionBinaryMagic) {
		fMajor = binary[kImageV1Major];
		fMinor = binary[kImageV1Minor];
		fRevision = binary[kImageV1Revision];
		fStage = kRelease;
		fStageNumber = 0;
		fBuildNumber = 0;
		valid = true;
	}
	else {
		valid = false;
	}
	return valid;
}

void CVersion::Write(CMedia& media) const {
	std::vector<std::uint8_t> binary;
	if (NeedsVersion2()) {
		binary.resize(kImageV2Size);
	}
	else {
		binary.resize(kImageV1Size);
	}
	Pack(&binary[0]);
	ASSERT(static_cast<RSMedia::TMediaPos>(binary.size()) <= std::numeric_limits<RSMedia::TMediaPos>::max());
	media.Write(static_cast<RSMedia::TMediaPos>(binary.size()), &binary[0]);
}

void CVersion::Pack2ByteMajorMinor(std::uint8_t binary[]) const {
	binary[0] = static_cast<std::uint8_t>(GetMajor());
	binary[1] = static_cast<std::uint8_t>(GetMinor());
}

void CVersion::Unpack2ByteMajorMinor(const std::uint8_t binary[]) {
	fMajor = binary[0];
	fMinor = binary[1];
	fRevision = 0;
	fStage = kRelease;
	fStageNumber = 0;
	fBuildNumber = 0;
}

bool CVersion::NeedsVersion2() const {
	bool needsVersion2 = false;
	if ((fStage != kRelease) || (fStageNumber != 0) || (fBuildNumber != 0)) {
		needsVersion2 = true;
	}
	if ((fMajor > 254) || (fMinor > 254) || (fRevision > 254)) {
		needsVersion2 = true;
	}
	return needsVersion2;
}

} // RSMedia
