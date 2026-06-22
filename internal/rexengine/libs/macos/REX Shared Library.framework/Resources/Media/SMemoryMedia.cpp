#include "StdInclude.h"

#include "SMemoryMedia.h"

#include "Core/Bacteria/Bacteria.h"

#include "Core/Debug/CheckedCast.h"
#include "SmugglerTypes.h"






namespace RSMedia {







////////////////////		CMemoryMedia






CMemoryMedia::CMemoryMedia(const CMedia& media) :
	CMedia(media),
	fMode(kOwnsBufferRW),
	fBuffer(NULL),
	fAllocatedBufferSize(media.GetSize()),
	fUsedBufferSize(media.GetSize())
{
	try {
		fBuffer=new std::uint8_t[CheckedCast<size_t>(fAllocatedBufferSize)];
		TMediaPos oldPos=media.GetCurrentPosition();
		media.Read(0,fUsedBufferSize,fBuffer);
		media.SetCurrentPosition(oldPos);
		CHK_INV;
	}
	catch(...){
		delete[] fBuffer;
		fBuffer = NULL;
		BRETHROW;
	}
}

CMemoryMedia::CMemoryMedia(const CMemoryMedia& media) :
	CMedia(media),
	fMode(kOwnsBufferRW),
	fBuffer(NULL),
	fAllocatedBufferSize(media.GetSize()),
	fUsedBufferSize(media.GetSize())
{
	try {
		fBuffer=new std::uint8_t[CheckedCast<size_t>(fAllocatedBufferSize)];
		TMediaPos oldPos=media.GetCurrentPosition();
		media.Read(0,fUsedBufferSize,fBuffer);
		media.SetCurrentPosition(oldPos);
		CHK_INV;
	}
	catch(...){
		delete[] fBuffer;
		fBuffer = NULL;
		BRETHROW;
	}
}



//lint -save -e1550
// JA: There's no need for a try-catch block in this function,
// there's nothing to clean up in case of an exception.
CMemoryMedia::CMemoryMedia(TMediaPos preAllocatedSize) :
	fMode(kOwnsBufferRW),
	fBuffer(NULL),
	fAllocatedBufferSize(preAllocatedSize),
	fUsedBufferSize(0)
{
	ASSERT(preAllocatedSize >=0);

	fBuffer=new std::uint8_t[CheckedCast<size_t>(fAllocatedBufferSize)];

	CHK_INV;
}
//lint -restore

CMemoryMedia::CMemoryMedia(TMediaPos bufferSize, const std::uint8_t buffer[]) :
	fMode(kReferencesBufferRO),
	fBuffer(const_cast<std::uint8_t*>(buffer)),
	fAllocatedBufferSize(bufferSize),
	fUsedBufferSize(bufferSize)
{
	ASSERT(bufferSize >=0);
	ASSERT(buffer != NULL);

	CHK_INV;
}

CMemoryMedia::CMemoryMedia(TMediaPos bufferSize,std::uint8_t buffer[]) :
	fMode(kReferencesBufferRW),
	fBuffer(buffer),
	fAllocatedBufferSize(bufferSize),
	fUsedBufferSize(bufferSize)
{
	ASSERT(bufferSize >=0);
	ASSERT(buffer != NULL);

	CHK_INV;
}

CMemoryMedia::~CMemoryMedia(){
	if(fMode==kOwnsBufferRW){
		delete[] fBuffer;
		fBuffer=NULL;
	}
	else{
		//This is not a memory leak, because some other media owns the memory pointed to by fBuffer
		//lint -save -e672
		fBuffer=NULL;
		//lint -restore
	}
}

#if DEBUG
void CMemoryMedia::CheckInvariant() const {
	ASSERT(fBuffer != NULL);
	ASSERT(fCurrentPosition >= 0);
	ASSERT(fCurrentPosition <= fUsedBufferSize);

/*
	switch(fMode){
		case kOwnsBufferRW:
			{
			}
			break;

		case kReferencesBufferRW:
			{
			}
			break;

		case kReferencesBufferRO:
			{
			}
			break;

		default:
			ASSERT(false);
			break;
	}
*/
}
#endif

void CMemoryMedia::GrowBufferIfNeeded(TMediaPos endPos){
	CHK_INV_SCOPE;
	ASSERT(endPos >=0);
	ASSERT(fMode==kOwnsBufferRW || fMode==kReferencesBufferRW);

	std::uint8_t* newBuffer=NULL;

	try{
		//	Grow buffer?
		if(endPos > fAllocatedBufferSize){
			//	You can't grow a buffer the CMemoryMedia doesn't own!
			ASSERT(kOwnsBufferRW);

			//	Grow so we can complete write request, but add 50% extra.
			auto growBytes = CheckedCast<RSMedia::TRawBytePos>(((endPos - fAllocatedBufferSize) * 3) / 2);
			//	Grow at least 25% beyond fAllocatedBufferSize
			auto minimumGrowSize = CheckedCast<RSMedia::TRawBytePos>(fAllocatedBufferSize / 4);
			if(minimumGrowSize < 100){
				minimumGrowSize=100;
			}
			if (growBytes < minimumGrowSize) {
				growBytes = minimumGrowSize;
			}
			TMediaPos newBufferSize = fAllocatedBufferSize + static_cast<TMediaPos>(growBytes);
			ASSERT(static_cast<TMediaPos>(newBufferSize) > fAllocatedBufferSize);

			newBuffer=new std::uint8_t[CheckedCast<std::size_t>(newBufferSize)];
			std::memcpy(newBuffer, fBuffer, CheckedCast<std::size_t>(fUsedBufferSize));
			delete[] fBuffer;
			fBuffer=NULL;

			fBuffer=newBuffer;
			newBuffer=NULL;
			fAllocatedBufferSize=newBufferSize;
		}
		ASSERT(fAllocatedBufferSize >=endPos);
	}
	catch(...){
		delete[] newBuffer;
		newBuffer=NULL;
		BRETHROW;
	}
}

const std::uint8_t* CMemoryMedia::GetBufferPtr() const{
	CHK_INV_SCOPE;
//	ASSERT(fMode==kOwnsBufferRW || fMode==kReferencesBufferRW);

	return fBuffer;
}

void CMemoryMedia::IMediaImplementation_ReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iDestinationBuffer) const {
	CHK_INV_SCOPE;
	ASSERT(iDestinationBuffer != NULL);
	ASSERT(iLength >= 0);
	ASSERT((iPosition + iLength) <= fUsedBufferSize);

	std::memcpy(iDestinationBuffer, fBuffer + iPosition, static_cast<std::size_t>(iLength));
}

void CMemoryMedia::IMediaImplementation_WriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iSourceBuffer) {
	CHK_INV_SCOPE;
	ASSERT(iSourceBuffer != NULL);
	ASSERT(iLength >= 0);
	ASSERT((fMode == kOwnsBufferRW) || (fMode == kReferencesBufferRW));

	TMediaPos endPos = iPosition + iLength;
	GrowBufferIfNeeded(endPos);

	//	Write to buffer.
	ASSERT(fAllocatedBufferSize >= endPos);
	std::memcpy(fBuffer + iPosition, iSourceBuffer, CheckedCast<std::size_t>(iLength));
	if (endPos > fUsedBufferSize) {
		fUsedBufferSize = endPos;
	}
}

TMediaPos CMemoryMedia::IMediaImplementation_GetLength() const {
	return fUsedBufferSize;
}

std::uint8_t* CMemoryMedia::WriteDirect(TMediaPos byteToWrite){
	CHK_INV_SCOPE;
	ASSERT(byteToWrite >=0);
	ASSERT(fMode==kOwnsBufferRW || fMode==kReferencesBufferRW);

	TMediaPos endPos = GetCurrentPosition() + byteToWrite;
	GrowBufferIfNeeded(endPos);

	//	Write to buffer.
	ASSERT(fAllocatedBufferSize >=endPos);
	if(endPos > fUsedBufferSize){
		fUsedBufferSize=endPos;
	}

	return fBuffer;
}



}	//	RSMedia
