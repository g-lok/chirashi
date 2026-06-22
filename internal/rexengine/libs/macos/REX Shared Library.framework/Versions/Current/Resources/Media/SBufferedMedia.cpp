#include "SBufferedMedia.h"
#include "Core/Test/TestPackage.h"
#include "SOwnedMemoryMedia.h"

namespace RSMedia {

CBufferedMedia::CBufferedMedia(CMedia& iParentMedia, TMediaPos iBufferSize)
  : fParentMedia(&iParentMedia),
	fParentReadOnlyMedia(nullptr),
	fBufferedDataType(EBufferedDataType::None),
	fBuffer(iBufferSize),
	fBufferedStartPos(-1),
	fBufferedSize(0),
	fParentMediaSize(kUnknownMediaSize)
{
    CHK_INV;
}

CBufferedMedia::CBufferedMedia(const CMedia& iParentMedia, TMediaPos iBufferSize)
  : fParentMedia(nullptr),
	fParentReadOnlyMedia(&iParentMedia),
	fBufferedDataType(EBufferedDataType::None),
	fBuffer(iBufferSize),
	fBufferedStartPos(-1),
	fBufferedSize(0),
	fParentMediaSize(iParentMedia.IMediaImplementation_GetLength())
{
    CHK_INV;
}

CBufferedMedia::~CBufferedMedia() {
    CHK_INV;

	try {
		Flush();
	}
	catch (...) {
		ASSERT(false);
	}
	
	fParentMedia = nullptr;
	fParentReadOnlyMedia = nullptr;
}

#if DEBUG
void CBufferedMedia::CheckInvariant() const {
	ASSERT(!fBuffer.empty());

	if (fParentMedia != nullptr) {
		ASSERT(fParentReadOnlyMedia == nullptr);
	} 
	else {
		ASSERT(fParentReadOnlyMedia != nullptr);
		ASSERT(fParentMediaSize >= 0);
	}

	if (fBufferedDataType != EBufferedDataType::None) {
		ASSERT(fBufferedSize > 0);
		ASSERT(fBufferedSize <= BufferCapacity());
		ASSERT(fBufferedStartPos >= 0);
	} 
	else {
		ASSERT(fBufferedSize == 0);
		ASSERT(fBufferedStartPos == -1);
	}
}
#endif // DEBUG

void CBufferedMedia::IMediaImplementation_ReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iBuffer) const {
	CHK_INV_SCOPE;
	ASSERT(iLength >= 0);
	ASSERT(iPosition >= 0);
	ASSERT(iBuffer != nullptr);

	if (iLength == 0) {
		return;
	}

	const_cast<CBufferedMedia*>(this)->Flush();

	// NBE: Bypass the buffer if read size is larger than the buffer size
	if (iLength > BufferCapacity()) {
		DoReadBytes(iPosition, iLength, iBuffer);
		return;
	}

	if (fBufferedDataType != EBufferedDataType::Read || iPosition < fBufferedStartPos || iPosition + iLength > fBufferedStartPos + fBufferedSize ) {
		const TMediaPos bytesToRead = std::min(BufferCapacity(), GetSize() - iPosition);

		// NBE: Invalidate the buffer here, so that we keep invariants if the read throws
		InvalidateBuffer();
		DoReadBytes(iPosition, bytesToRead, fBuffer.data());

		fBufferedDataType = EBufferedDataType::Read;
		fBufferedStartPos = iPosition;
		fBufferedSize = bytesToRead;
	}

	ASSERT(fBufferedStartPos >= 0);
	ASSERT(fBufferedSize > 0);
	ASSERT(fBufferedDataType == EBufferedDataType::Read);
	ASSERT(iPosition >= fBufferedStartPos);
	ASSERT(iLength <= fBufferedSize);
	const TMediaPos posInBuffer = iPosition - fBufferedStartPos;
	ASSERT(posInBuffer + iLength <= fBufferedSize);

	auto outputBuffer = static_cast<std::uint8_t*>(iBuffer);
	std::copy(fBuffer.begin() + posInBuffer, fBuffer.begin() + posInBuffer + iLength, outputBuffer);
}

void CBufferedMedia::DoReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iBuffer) const {
	if (fParentMedia != nullptr) {
		fParentMedia->IMediaImplementation_ReadBytes(iPosition, iLength, iBuffer);
	}
	else {
		fParentReadOnlyMedia->IMediaImplementation_ReadBytes(iPosition, iLength, iBuffer);
	}
}

void CBufferedMedia::InvalidateBuffer() const {
	fBufferedDataType = EBufferedDataType::None;
	fBufferedStartPos = -1;
	fBufferedSize = 0;
}

void CBufferedMedia::IMediaImplementation_WriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iBuffer) {
	CHK_INV_SCOPE;
	ASSERT(iLength >= 0);
	ASSERT(iPosition >= 0);
	ASSERT(iBuffer != nullptr);

	if (iLength == 0) {
		return;
	}

	const bool writeWouldOverflowBuffer = iLength > BufferCapacity() ||
		(fBufferedDataType == EBufferedDataType::Write && (iPosition + iLength) >= (fBufferedStartPos + BufferCapacity()));

	if (writeWouldOverflowBuffer) {
		Flush();
		DoWriteBytes(iPosition, iLength, iBuffer);
		return;
	}

	auto inputBuffer = static_cast<const std::uint8_t*>(iBuffer);
	const TMediaPos bufferEndPosition = fBufferedStartPos + fBufferedSize;
	if (fBufferedDataType == EBufferedDataType::Write && iPosition == bufferEndPosition) {
		std::copy(inputBuffer, inputBuffer + iLength, fBuffer.begin() + fBufferedSize);
		fBufferedSize += iLength;
	} else {
		Flush();
		std::copy(inputBuffer, inputBuffer + iLength, fBuffer.begin());
		fBufferedSize = iLength;
		fBufferedDataType = EBufferedDataType::Write;
		fBufferedStartPos = iPosition;
	}

	ASSERT(fBufferedDataType == EBufferedDataType::Write);
	ASSERT((fBufferedStartPos + fBufferedSize) == (iPosition + iLength));
}

void CBufferedMedia::Flush() {
	CHK_INV_SCOPE;

	if (fBufferedDataType == EBufferedDataType::Write) {
		ASSERT(fParentMedia != nullptr);
		DoWriteBytes(fBufferedStartPos, fBufferedSize, fBuffer.data());
		InvalidateBuffer();
	}
}

void CBufferedMedia::DoWriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iBuffer) {
	ASSERT(fParentMedia != nullptr);
	fParentMedia->IMediaImplementation_WriteBytes(iPosition, iLength, iBuffer);
	fParentMediaSize = kUnknownMediaSize;
}

TMediaPos CBufferedMedia::IMediaImplementation_GetLength() const {
    CHK_INV;
	if (fParentMediaSize == kUnknownMediaSize) {
		ASSERT(fParentMedia != nullptr);
		fParentMediaSize = fParentMedia->IMediaImplementation_GetLength();
	}

	if (fBufferedDataType == EBufferedDataType::Write) {
		return std::max(fParentMediaSize, fBufferedStartPos + fBufferedSize);
	}
	else {
		return fParentMediaSize;
	}
}

#if DEBUG
namespace {
struct BufferedReaderTestData {
	BufferedReaderTestData(std::size_t size)
	{
		fData.reserve(size);
		for (std::uint16_t c = 0; c < size; c++) {
			fData.push_back(static_cast<std::uint8_t>(std::rand() & 0xff));
		}
	}

	std::vector<std::uint8_t> Subrange(TMediaPos iPosition, TMediaPos iLength) const {
		ASSERT(iPosition + iLength <= static_cast<TMediaPos>(fData.size()));
		return std::vector<std::uint8_t>(fData.begin() + iPosition, fData.begin() + iPosition + iLength);
	}

	std::uint8_t* GetDataPointer(std::size_t offset)
	{
		ASSERT(offset < fData.size());
		return &fData[offset];
	}

	std::vector<std::uint8_t> fData;
};
}

QUICKTEST_SINGLETHREAD("Media", "CBufferedMedia::Write") {
	auto testData = BufferedReaderTestData(128);

	COwnedMemoryMedia memoryMedia;
	CBufferedMedia sut(memoryMedia, 8);

	auto readData = [&](TMediaPos iPosition, TMediaPos iLength) {
		std::vector<std::uint8_t> output(iLength);
		sut.Read(iPosition, iLength, output.data());
		return output;
	};

	TEST_VERIFY(sut.GetSize() == 0);
	TEST_VERIFY(sut.GetCurrentPosition() == 0);

	// Write at beginning
	sut.Write(0, 1, testData.GetDataPointer(0));
	TEST_VERIFY(sut.GetSize() == 1);
	TEST_VERIFY(readData(0, 1) == testData.Subrange(0, 1));

	// Out of order writes
	sut.Write(0, 3, testData.GetDataPointer(20)); // Fill with some random data
	sut.Write(2, 1, testData.GetDataPointer(2));
	sut.Write(0, 1, testData.GetDataPointer(0));
	sut.Write(1, 1, testData.GetDataPointer(1));
	TEST_VERIFY(sut.GetSize() == 3);
	TEST_VERIFY(readData(0, 3) == testData.Subrange(0, 3));

	// Two contiguous writes with size equal to the total buffer size
	sut.Write(1, 4, testData.GetDataPointer(1));
	sut.Write(5, 4, testData.GetDataPointer(5));
	TEST_VERIFY(sut.GetSize() == 9);
	TEST_VERIFY(readData(0, 9) == testData.Subrange(0, 9));

	// Two contiguous writes with size larger than total buffer size
	sut.Write(0, 4, testData.GetDataPointer(10));
	sut.Write(4, 6, testData.GetDataPointer(14));
	TEST_VERIFY(sut.GetSize() == 10);
	TEST_VERIFY(readData(0, 10) == testData.Subrange(10, 10));

	// Single write larger than the total buffer size
	sut.Write(10, 24, testData.GetDataPointer(0));
	TEST_VERIFY(sut.GetSize() == 34);
	TEST_VERIFY(readData(0, 10) == testData.Subrange(10, 10));
	TEST_VERIFY(readData(10, 24) == testData.Subrange(0, 24));
}

QUICKTEST_SINGLETHREAD("Media", "CBufferedMedia::Read") {
	constexpr TMediaPos testDataSize = 128;
	auto testData = BufferedReaderTestData(testDataSize);

	const COwnedMemoryMedia memoryMedia(testDataSize, testData.GetDataPointer(0));
	CBufferedMedia sut(memoryMedia, 8);

	auto readData = [&](TMediaPos iPosition, TMediaPos iLength) {
		std::vector<std::uint8_t> output(iLength);
		sut.Read(iPosition, iLength, output.data());
		return output;
	};

	TEST_VERIFY(sut.GetSize() == testDataSize);

	// Sequential reads of different size
	for (TMediaPos chunkSize = 1; chunkSize < 20; ++chunkSize) {
		std::vector<std::uint8_t> buffer(chunkSize, 0);
		for (TMediaPos offset = 0; offset <= testDataSize - chunkSize; offset += chunkSize) {
			TEST_VERIFY(readData(offset, chunkSize) == testData.Subrange(offset, chunkSize));
		}
	}

	// Random access reads
	TEST_VERIFY(readData(0, 3) == testData.Subrange(0, 3));
	TEST_VERIFY(readData(40, 1) == testData.Subrange(40, 1));
	TEST_VERIFY(readData(120, 8) == testData.Subrange(120, 8));
	TEST_VERIFY(readData(19, 100) == testData.Subrange(19, 100));
}

#endif

}
