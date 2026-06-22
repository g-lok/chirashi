#include "StdInclude.h"

#include "SBinary.h"

#include "Core/Test/TestPackage.h"





namespace RSMedia {


#if DEBUG

class CBinaryUnitTest : public RSTest::CFixture {
	public: CBinaryUnitTest(std::string description) :
		RSTest::CFixture(description){}

	public: void UnitTest64BitConversions();
	public: void UnitTestIEEE80();
	public: void UnitTestFloatingPointConversions();
};


void CBinaryUnitTest::UnitTest64BitConversions(){
	SCOPED_TRACE("64 bit conversions");
	try{
		//	Try LongsToUInt64(). Note that LongsToUInt64() itself verifies it can convert back.
		{
			{
				std::uint64_t a64=LongsToUInt64(0x00000000,0x00000000);
				ASSERT(a64==0x0000000000000000);
			}

			{
				std::uint64_t b64=LongsToUInt64(0xffffffff,0xffffffff);
#if WINDOWS
				ASSERT(b64==0xffffffffffffffff);
#else
				ASSERT(b64==0xffffffffffffffffULL);
#endif
			}
			{
				std::uint64_t c64=LongsToUInt64(0xffffffff,0x00000000);
#if WINDOWS
				ASSERT(c64==0xffffffff00000000);
#else
				ASSERT(c64==0xffffffff00000000ULL);
#endif
			}
			{
				std::uint64_t d64=LongsToUInt64(0x00000000,0xffffffff);
				ASSERT(d64==0x00000000ffffffff);
			}
			{
				std::uint64_t e64=LongsToUInt64(0x10000000,0x20000000);
#if WINDOWS
				ASSERT(e64==0x1000000020000000);
#else
				ASSERT(e64==0x1000000020000000ULL);
#endif
			}
		}

		//	Try UInt64ToLongs(). Note that UInt64ToLongs() itself verifies it can convert back.
		{
			{
				std::uint32_t hi=0;
				std::uint32_t low=0;
				UInt64ToLongs(0x0000000000000000,hi,low);
				ASSERT(hi==0x00000000);
				ASSERT(low==0x00000000);
			}
			{
				std::uint32_t hi=0;
				std::uint32_t low=0;
#if WINDOWS
				UInt64ToLongs(0xffffffffffffffff,hi,low);
#else
				UInt64ToLongs(0xffffffffffffffffULL,hi,low);
#endif
				ASSERT(hi==0xffffffff);
				ASSERT(low==0xffffffff);
			}
			{
				std::uint32_t hi=0;
				std::uint32_t low=0;
#if WINDOWS
				UInt64ToLongs(0xffffffff00000000,hi,low);
#else
				UInt64ToLongs(0xffffffff00000000ULL,hi,low);
#endif
				ASSERT(hi==0xffffffff);
				ASSERT(low==0x00000000);
			}
			{
				std::uint32_t hi=0;
				std::uint32_t low=0;
				UInt64ToLongs(0x00000000ffffffff,hi,low);
				ASSERT(hi==0x00000000);
				ASSERT(low==0xffffffff);
			}
			{
				std::uint32_t hi=0;
				std::uint32_t low=0;
#if WINDOWS
				UInt64ToLongs(0x2000000010000000,hi,low);
#else
				UInt64ToLongs(0x2000000010000000ULL,hi,low);
#endif
				ASSERT(hi==0x20000000);
				ASSERT(low==0x10000000);
			}
		}
	}
	catch(...){
		ASSERT(false);
	}
}


void CBinaryUnitTest::UnitTestIEEE80(){
	SCOPED_TRACE("IEEE80");
	try{
		std::uint8_t binary[100];

		{
			double value1 = 0.123456;
			PackIEEE80Big( binary, value1);
			double val = UnpackIEEE80Big(binary);
			ASSERT(val==value1);
		}
		{
			double value1 = -0.123456;
			PackIEEE80Big( binary, value1);
			double val = UnpackIEEE80Big(binary);
			ASSERT(val==value1);
		}
		{
			double value1 = 0.0;
			PackIEEE80Big( binary, value1);
			double val = UnpackIEEE80Big(binary);
			ASSERT(val==value1);
		}
		{
			double value1 = 987654321.0;
			PackIEEE80Big( binary, value1);
			double val = UnpackIEEE80Big(binary);
			ASSERT(val==value1);
		}
		{
			double value1 = -987654321.0;
			PackIEEE80Big( binary, value1);
			double val = UnpackIEEE80Big(binary);
			ASSERT(val==value1);
		}
	}
	catch(...){
		ASSERT(false);
	}
}


void CBinaryUnitTest::UnitTestFloatingPointConversions(){
	SCOPED_TRACE("UnitTestFloatingPointConversions");
	try{
		// Truncating
		{
			float floatValue = 1.2f;
			std::int32_t truncValue = RSMedia::TruncFloatToInt32(floatValue);
			ASSERT(truncValue == 1 );
		}
		{
			float floatValue = 1.7f;
			std::int32_t truncValue = RSMedia::TruncFloatToInt32(floatValue);
			ASSERT(truncValue == 1 );
		}
		{
			float floatValue = -1.2f;
			std::int32_t truncValue = RSMedia::TruncFloatToInt32(floatValue);
			ASSERT(truncValue == -1 );
		}
		{
			float floatValue = -1.7f;
			std::int32_t truncValue = RSMedia::TruncFloatToInt32(floatValue);
			ASSERT(truncValue == -1 );
		}

		// Rounding
		{
			float floatValue = 1.2f;
			std::int32_t truncValue = RSMedia::RoundFloatToInt32(floatValue);
			ASSERT(truncValue == 1.0 );
		}
		{
			float floatValue = 1.7f;
			std::int32_t truncValue = RSMedia::RoundFloatToInt32(floatValue);
			ASSERT(truncValue == 2.0 );
		}
		{
			float floatValue = -1.2f;
			std::int32_t truncValue = RSMedia::RoundFloatToInt32(floatValue);
			ASSERT(truncValue == -1.0 );
		}
		{
			float floatValue = -1.7f;
			std::int32_t truncValue = RSMedia::RoundFloatToInt32(floatValue);
			ASSERT(truncValue == -2.0 );
		}
	}
	catch(...){
		ASSERT(false);
	}
}




static class CBinaryTestSuite : public RSTest::CTestSuite {
	public: CBinaryTestSuite() : RSTest::CTestSuite("Core::SBinaryUnitTest") {

			AddTest("UnitTest64BitConversions()", &CBinaryUnitTest::UnitTest64BitConversions);
			AddTest("UnitTestIEEE80()", &CBinaryUnitTest::UnitTestIEEE80);
			AddTest("UnitTestFloatingPointConversions()", &CBinaryUnitTest::UnitTestFloatingPointConversions);

			RSTest::RegisterSuite(this);
			}
}sMediaSmugglerBinaryTestSuite;


#endif // DEBUG



}	//	RSMedia


