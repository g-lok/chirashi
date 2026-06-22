#include "StdInclude.h"

#include "SOwnedMemoryMedia.h"

#include "Core/Debug/CheckedCast.h"
#include "Core/Test/TestPackage.h"






namespace RSMedia {



COwnedMemoryMedia::COwnedMemoryMedia(const CMedia& iMedia) :
	CMedia(iMedia),
	fBuffer()
{
	fBuffer.resize(CheckedCast<std::size_t>(iMedia.GetSize()));
	if (fBuffer.size() > 0) {
		iMedia.IMediaImplementation_ReadBytes(0, fBuffer.size(), &fBuffer[0]);
	}

	CHK_INV;
}

COwnedMemoryMedia::COwnedMemoryMedia(const COwnedMemoryMedia& iMedia):
	CMedia(iMedia),
	fBuffer()
{
	fBuffer.resize(CheckedCast<std::size_t>(iMedia.GetSize()));
	if (fBuffer.size() > 0) {
		iMedia.IMediaImplementation_ReadBytes(0, fBuffer.size(), &fBuffer[0]);
	}

	CHK_INV;
}


//lint -save -e1550
// JA: There's no need for a try-catch block in this function,
// there's nothing to clean up in case of an exception.
COwnedMemoryMedia::COwnedMemoryMedia(TMediaPos iPreAllocatedSize) :
	fBuffer()
{
	ASSERT(iPreAllocatedSize >= 0);

	fBuffer.reserve(CheckedCast<std::size_t>(iPreAllocatedSize));

	CHK_INV;
}
//lint -restore


//lint -save -e1550
// JA: There's no need for a try-catch block in this function,
// there's nothing to clean up in case of an exception.
COwnedMemoryMedia::COwnedMemoryMedia() :
	fBuffer()
{
	CHK_INV;
}
//lint -restore


//lint -save -e1550
// JA: There's no need for a try-catch block in this function,
// there's nothing to clean up in case of an exception.
COwnedMemoryMedia::COwnedMemoryMedia(TMediaPos iBufferSize, const std::uint8_t iBuffer[]) :
	fBuffer()
{
	ASSERT(iBufferSize >= 0);
	ASSERT(iBuffer != NULL);

	fBuffer.resize(CheckedCast<std::size_t>(iBufferSize));
	if (iBufferSize > 0) {
		std::memcpy(&fBuffer[0], iBuffer, CheckedCast<std::size_t>(iBufferSize));
	}

	CHK_INV;
}
//lint -restore

COwnedMemoryMedia::~COwnedMemoryMedia() {
}

#if DEBUG
void COwnedMemoryMedia::CheckInvariant() const {
	ASSERT(fCurrentPosition >= 0);
	ASSERT(fCurrentPosition <= static_cast<TMediaPos>(fBuffer.size()));
}
#endif // DEBUG

void COwnedMemoryMedia::GrowBufferIfNeeded(TMediaPos iEndPos) {
	CHK_INV_SCOPE;
	ASSERT(iEndPos >= 0);

	//	Grow buffer?
	if (iEndPos > static_cast<TMediaPos>(fBuffer.capacity())) {
		//	Grow so we can complete write request, but add 50% extra.
		TRawBytePos growBytes = CheckedCast<RSMedia::TRawBytePos>(((iEndPos - fBuffer.capacity()) * 3) / 2);
		//	Grow at least 25% beyond fBuffer.capacity()
		TRawBytePos minimumGrowSize = CheckedCast<TRawBytePos>(fBuffer.capacity() / 4);
		//	Or at least 100 bytes
		if(minimumGrowSize < 100){
			minimumGrowSize = 100;
		}
		if (growBytes < minimumGrowSize) {
			growBytes = minimumGrowSize;
		}
		TRawBytePos newBufferSize = CheckedCast<TRawBytePos>(fBuffer.capacity() + growBytes);
		ASSERT(newBufferSize > fBuffer.capacity());

		fBuffer.reserve(newBufferSize);
	}
	ASSERT(static_cast<TMediaPos>(fBuffer.capacity()) >= iEndPos);

	if (iEndPos > static_cast<TMediaPos>(fBuffer.size())) {
		fBuffer.resize(CheckedCast<std::size_t>(iEndPos));
	}
}

const std::uint8_t* COwnedMemoryMedia::GetBufferPtr() const {
	CHK_INV;
	ASSERT(fBuffer.size() > 0);
	return &fBuffer[0];
}

void COwnedMemoryMedia::IMediaImplementation_ReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iDestinationBuffer) const {
	// DE: No need to do CheckedCast here since we already have verified sizes etc. with other ASSERTs

	CHK_INV_SCOPE;
	ASSERT(iDestinationBuffer != NULL);
	ASSERT(iLength >= 0);
	ASSERT((iPosition + iLength) <= static_cast<TMediaPos>(fBuffer.size()));

	if (iLength > 0) {
		std::memcpy(iDestinationBuffer, &fBuffer[static_cast<std::size_t>(iPosition)], static_cast<std::size_t>(iLength));
	}
}

void COwnedMemoryMedia::IMediaImplementation_WriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iSourceBuffer) {
	// DE: No need to do CheckedCast here since we already have verified sizes etc. with other ASSERTs

	CHK_INV_SCOPE;
	ASSERT(iSourceBuffer != NULL);
	ASSERT(iLength >= 0);

	if(iLength > 0){
		{
			TMediaPos endPos = iPosition + iLength;
			GrowBufferIfNeeded(endPos);
			ASSERT(static_cast<TMediaPos>(fBuffer.size()) >= endPos);
		}

		//	Write to buffer.
		ASSERT((iPosition + iLength) <= static_cast<TMediaPos>(fBuffer.size()));
		std::memcpy(&fBuffer[static_cast<std::size_t>(iPosition)], iSourceBuffer, static_cast<std::size_t>(iLength));
	}
}

TMediaPos COwnedMemoryMedia::IMediaImplementation_GetLength() const {
	CHK_INV;
	return fBuffer.size();
}

std::uint8_t* COwnedMemoryMedia::WriteDirect(TMediaPos iBytesToWrite){
	CHK_INV_SCOPE;
	ASSERT(iBytesToWrite >= 0);

	TMediaPos endPos = GetCurrentPosition() + iBytesToWrite;

	//	"Write" to buffer
	GrowBufferIfNeeded(endPos);
	ASSERT(static_cast<TMediaPos>(fBuffer.size()) >= endPos);

	return &fBuffer[0];
}


#if DEBUG

class COwnedMemoryMediaUnitTest : public RSTest::CFixture {
	public: COwnedMemoryMediaUnitTest(std::string description) :
		RSTest::CFixture(description){}

	public: void UnitTestOwnedMemoryMedia();
};

static class COwnedMemoryMediaTestSuite : public RSTest::CTestSuite {
	public: COwnedMemoryMediaTestSuite() : RSTest::CTestSuite("Core::SOwnedMemoryMediaUnitTest") {

			AddTest("UnitTestOwnedMemoryMedia()", &COwnedMemoryMediaUnitTest::UnitTestOwnedMemoryMedia);

			RSTest::RegisterSuite(this);
		}
}sMediaSmugglerOwnedMemeoryMediaTestSuite;

void COwnedMemoryMediaUnitTest::UnitTestOwnedMemoryMedia(){
	COwnedMemoryMedia a(10);
	ASSERT(a.GetCurrentPosition()==0);
	ASSERT(a.GetSize()==0);


	std::uint8_t bytes[1000];
	for(std::uint16_t c=0 ; c < 1000 ; c++){
		bytes[c]=static_cast<std::uint8_t>(std::rand() & 0xff);
	}

	//	Write small chunks random-access and verify.
	{
		a.Write(0,2,&bytes[0]);
		ASSERT(a.GetCurrentPosition()==2);
		ASSERT(a.GetSize()==2);

		a.Write(0,3,&bytes[2]);
		ASSERT(a.GetCurrentPosition()==3);
		ASSERT(a.GetSize()==3);

		//	This will grow the internal buffer.
		a.Write(3,10,&bytes[5]);
		ASSERT(a.GetCurrentPosition()==13);
		ASSERT(a.GetSize()==13);
	}

	//	Write big chunks random-access and verify.
	{
		a.Write(1,999,&bytes[0]);
		ASSERT(a.GetCurrentPosition()==1000);
		ASSERT(a.GetSize()==1000);

		std::uint8_t temp[999];
		a.Read(1,999,temp);
		ASSERT(a.GetCurrentPosition()==1000);
		ASSERT(a.GetSize()==1000);
		ASSERT(std::memcmp(temp,&bytes[0],999)==0);
	}

	//	Try copy constructor.
	{
		//	Position of original is copied to new media and not changed in old.
		a.SetCurrentPosition(13);
		COwnedMemoryMedia b(a);
		ASSERT(b.GetCurrentPosition()==a.GetCurrentPosition());
		ASSERT(b.GetSize()==a.GetSize());

		std::uint8_t tempA[1000];
		a.Read(0,1000,tempA);
		std::uint8_t tempB[1000];
		b.Read(0,1000,tempB);
		ASSERT(std::memcmp(tempA,tempB,1000)==0);
	}

	//	Try COwnedMemoryMedia(TMediaPos size, const std::uint8_t buffer[]) constructor.
	{
		//	Position of original is copied to new media and not changed in old.
		COwnedMemoryMedia b(500,bytes);
		ASSERT(b.GetCurrentPosition()==0);
		ASSERT(b.GetSize()==500);

		std::uint8_t temp[500];
		b.Read(0,500,temp);
		ASSERT(std::memcmp(temp,bytes,500)==0);
	}
}

#endif	//	DEBUG


}	//	RSMedia


