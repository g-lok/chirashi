#include "StdInclude.h"

#if DEBUG

#include "SVersion.h"
#include "SVersionUtils.h"

#include "Core/Test/TestPackage.h"

namespace RSMedia {
namespace RSInternal {

/////////////////////		Unit-tests

class CVersionUnitTest : public RSTest::CFixture {
	public: CVersionUnitTest(std::string description) :
		RSTest::CFixture(description){}

	public: void UnitTestVersion();
};

static class CVersionTestSuite : public RSTest::CTestSuite {
	public: CVersionTestSuite() : RSTest::CTestSuite("Core::SVersionUnitTest") {

			AddTest("UnitTestVersion()", &CVersionUnitTest::UnitTestVersion);

			RSTest::RegisterSuite(this);
		}
}sMediaSmugglerVersionTestSuite;


void CVersionUnitTest::UnitTestVersion(){
	try{
		TRACE("CVersion::UnitTest");
		CVersion kOldestCompatibleVersion(0,0,9);
		CVersion kCurrentVersion(1,1,0);

		CVersion kTooOld(0,0,2);
		CVersion kNewerButSameMajor(1,2,3);
		CVersion kTooNew(2,2,3);

		ASSERT(!kTooOld.IsSupported(kOldestCompatibleVersion,kCurrentVersion));
		ASSERT(kNewerButSameMajor.IsSupported(kOldestCompatibleVersion,kCurrentVersion));
		ASSERT(!kTooNew.IsSupported(kOldestCompatibleVersion,kCurrentVersion));

		RSText::TString str = FormatVersionAsStringUS(kOldestCompatibleVersion);
		ASSERT(str == RSText::FromASCII("0.0.9"));
		CVersion oldestFromStr = ParseVersionStringUS(str);
		ASSERT(oldestFromStr.IsEqual(kOldestCompatibleVersion));

		str = FormatVersionAsStringUS(kCurrentVersion);
		ASSERT(str == RSText::FromASCII("1.1"));
		CVersion currentFromStr=ParseVersionStringUS(str);
		ASSERT(currentFromStr.IsEqual(kCurrentVersion));

		CVersion develop(1, 3, 6, CVersion::kDevelopment, 7);
		str = FormatVersionAsStringUS(develop);
		ASSERT(str == RSText::FromASCII("1.3.6d7"));
		CVersion developFromStr=ParseVersionStringUS(str);
		ASSERT(developFromStr.IsEqual(develop));

		CVersion alpha(1, 3, 6, CVersion::kAlpha, 7);
		str = FormatVersionAsStringUS(alpha);
		ASSERT(str == RSText::FromASCII("1.3.6a7"));
		CVersion alphaFromStr=ParseVersionStringUS(str);
		ASSERT(alphaFromStr.IsEqual(alpha));

		CVersion beta(1, 3, 6, CVersion::kBeta, 7);
		str = FormatVersionAsStringUS(beta);
		ASSERT(str == RSText::FromASCII("1.3.6b7"));
		CVersion betaFromStr=ParseVersionStringUS(str);
		ASSERT(betaFromStr.IsEqual(beta));

		CVersion release(1, 3, 6, CVersion::kRelease, 7);
		str = FormatVersionAsStringUS(release);
		ASSERT(str == RSText::FromASCII("1.3.6f7"));
		CVersion releaseFromStr = ParseVersionStringUS(str);
		ASSERT(releaseFromStr.IsEqual(release));

		CVersion empty = ParseVersionStringUS(RSText::TString());
		ASSERT(empty.IsEqual(CVersion(0,0)));

		CVersion na = ParseVersionStringUS(RSText::FromASCII("n/a"));
		ASSERT(na.IsEqual(CVersion(0,0)));

		//return true;
	}
	catch(...){
		ASSERT(false);
		//return false;
	}
}


} // RSInternal

} // RSMedia


#endif // DEBUG
