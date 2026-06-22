#pragma once

#include "Core/Text/TextPackage.h"

#include <cstdint>

#if MAC
    struct NumVersion;
#endif // MAC

namespace RSMedia {

class CMedia;

class CVersion {
public:
	enum EStage {
		kDevelopment = 1,
		kAlpha = 2,
		kBeta = 3,
		kRelease = 4
	};

	constexpr CVersion(std::uint32_t major, std::uint32_t minor);
	constexpr CVersion(std::uint32_t major, std::uint32_t minor, std::uint32_t revision);
	constexpr CVersion(std::uint32_t major, std::uint32_t minor, std::uint32_t revision, EStage stage, std::uint32_t stageNumber);
	constexpr CVersion(std::uint32_t major, std::uint32_t minor, std::uint32_t revision, EStage stage, std::uint32_t stageNumber, std::uint32_t buildNumber);
	explicit CVersion(const RSMedia::CMedia& media);

	std::uint32_t GetMajor() const;
	std::uint32_t GetMinor() const;
	std::uint32_t GetRevision() const;
	EStage GetStage() const;
	std::uint32_t GetStageNumber() const;
	std::uint32_t GetBuildNumber() const;

	enum EComparisonResult {
		kOtherIsOlder,
		kEqual,
		kOtherIsNewer
	};
	EComparisonResult Compare(const CVersion& other) const;
	bool operator<(const CVersion& o) const;

	// Version is null when major, minor, revision, stage and build numbers are zero and stage is kRelease.
	// Such a version has no numbers when used with GetString**()
	bool IsNull() const;

	//	Used mainly when checking if we can use an API.
	//	Returns true if this version is not older than "requiredVersion".
	bool IsNewEnough(const CVersion& requiredVersion) const;

	//	These functions are old. It's recommended you use IsCompatible() instead.
	bool IsEqual(const CVersion& requiredVersion) const;

    inline bool operator==(const CVersion& other) const {
        return IsEqual(other);
    }
    inline bool operator!=(const CVersion& other) const {
        return !IsEqual(other);
    }

	//	Returns true if version is not older than "oldestSupported" and it's major
	//	version is not newer than the "current" major version.
	bool IsSupported(const CVersion& oldestSupported, const CVersion& current) const;

	//	The version number rules are: when major version is the same the files are compatible.
	bool IsCompatible(const CVersion& current) const;

	void Pack(std::uint8_t binary[]) const;
	bool Unpack(const std::uint8_t binary[]);

	void Write(RSMedia::CMedia& media) const;

	// Byte 0 = major, byte 1 = minor.
	void Pack2ByteMajorMinor(std::uint8_t binary[]) const;
	void Unpack2ByteMajorMinor(const std::uint8_t binary[]);


	// Internals.
	private: enum EImageV1 {
		kImageV1Magic = 0,	// 0xbc
		kImageV1Major = 1,
		kImageV1Minor = 2,
		kImageV1Revision = 3,
		kImageV1Reserved = 4,
			kImageV1Size = 5
	};
	private: enum EImageV2 {
		kImageV2Magic = 0,	// 0xbc
		kImageV2DummyMajor = 1, // 0xff
		kImageV2DummyMinor = 2, // 0xff
		kImageV2DummyRevision = 3, // 0xff
		kImageV2DummyReserved = 4, // 0xff
		kImageV2Major = 5,
		kImageV2Minor = 9,
		kImageV2Revision = 13,
		kImageV2Stage = 17,
		kImageV2StageNumber = 21,
		kImageV2BuildNumber = 25,
			kImageV2Size = 29
	};
	private: bool NeedsVersion2() const;

	// Data.
	private: std::uint32_t fMajor;
	private: std::uint32_t fMinor;
	private: std::uint32_t fRevision;
	private: EStage fStage;
	private: std::uint32_t fStageNumber;
	private: std::uint32_t fBuildNumber;
};



constexpr CVersion::CVersion(std::uint32_t major, std::uint32_t minor) :
	fMajor(major),
	fMinor(minor),
	fRevision(0),
	fStage(kRelease),
	fStageNumber(0),
	fBuildNumber(0)
{
}

constexpr CVersion::CVersion(std::uint32_t major, std::uint32_t minor, std::uint32_t revision) :
	fMajor(major),
	fMinor(minor),
	fRevision(revision),
	fStage(kRelease),
	fStageNumber(0),
	fBuildNumber(0)
{
}

constexpr CVersion::CVersion(std::uint32_t major, std::uint32_t minor, std::uint32_t revision, EStage stage, std::uint32_t stageNumber) :
	fMajor(major),
	fMinor(minor),
	fRevision(revision),
	fStage(stage),
	fStageNumber(stageNumber),
	fBuildNumber(0)
{
}

constexpr CVersion::CVersion(std::uint32_t major, std::uint32_t minor, std::uint32_t revision, EStage stage, std::uint32_t stageNumber, std::uint32_t buildNumber) :
	fMajor(major),
	fMinor(minor),
	fRevision(revision),
	fStage(stage),
	fStageNumber(stageNumber),
	fBuildNumber(buildNumber)
{
}

}	//	RSMedia

