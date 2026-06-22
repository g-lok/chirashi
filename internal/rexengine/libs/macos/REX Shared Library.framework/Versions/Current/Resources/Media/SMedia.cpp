#include "StdInclude.h"

#include "SMedia.h"

#include "Core/Bacteria/Bacteria.h"
#include <cmath>
#include "Core/Debug/CheckedCast.h"

#include <memory>
#include "SBinary.h"


namespace RSMedia {


CMedia::CMedia() :
	fCurrentPosition(0)
{
	CHK_INV;
}

CMedia::CMedia(const CMedia& iOther) :
	fCurrentPosition(iOther.fCurrentPosition)
{
	CHK_INV;
}

CMedia::~CMedia() {
	CHK_INV;
}

CMedia& CMedia::operator=(const CMedia& iOther) {
	CHK_INV_SCOPE;
	if (this != &iOther) {
		fCurrentPosition = iOther.fCurrentPosition;
	}
	return *this;
}

#if DEBUG
void CMedia::CheckInvariant() const {
}
#endif // DEBUG


TMediaPos CMedia::GetSize() const {
	CHK_INV;
	return IMediaImplementation_GetLength();
}

void CMedia::Read(TMediaPos iSourcePosition, TMediaPos iSize, void* iDestinationBuffer) const {
	CHK_INV_SCOPE;
	ASSERT(iSourcePosition < GetSize());
	ASSERT(iSize >= 0);
	ASSERT((iSourcePosition + iSize) <= GetSize());
	ASSERT(iDestinationBuffer != NULL);

	IMediaImplementation_ReadBytes(iSourcePosition, iSize, iDestinationBuffer);
	fCurrentPosition = iSourcePosition + iSize;
	CheckThreadChangingPosition();
}

void CMedia::Peek(TMediaPos iSourcePosition, TMediaPos iSize, void* iDestinationBuffer) const {
	CHK_INV_SCOPE;
	ASSERT(iSourcePosition < GetSize());
	ASSERT(iSize >= 0);
	ASSERT((iSourcePosition + iSize) <= GetSize());
	ASSERT(iDestinationBuffer != NULL);

	IMediaImplementation_ReadBytes(iSourcePosition, iSize, iDestinationBuffer);
}

void CMedia::Read(TMediaPos iSize, void* iDestinationBuffer) const {
	CHK_INV_SCOPE;
	ASSERT(iSize >= 0);
	// JP: This isn't a programming error, it's a runtime error. For now, let the implementor of IMediaImplementation_ReadBytes() handle it
//	ASSERT((fCurrentPosition + iSize) <= GetSize());
	ASSERT(iDestinationBuffer != NULL);

	IMediaImplementation_ReadBytes(fCurrentPosition, iSize, iDestinationBuffer);
	fCurrentPosition += iSize;
	CheckThreadChangingPosition();
}

void CMedia::Write(TMediaPos iDestinationPosition, TMediaPos iSize, const void* iSourceBuffer) {
	CHK_INV_SCOPE;
	ASSERT(iDestinationPosition <= GetSize());
	ASSERT(iSize >= 0);
	ASSERT(iSourceBuffer != NULL);

	IMediaImplementation_WriteBytes(iDestinationPosition, iSize, iSourceBuffer);
	fCurrentPosition = iDestinationPosition + iSize;
	CheckThreadChangingPosition();
}

void CMedia::Write(TMediaPos iSize, const void* iSourceBuffer) {
	CHK_INV_SCOPE;
	ASSERT(fCurrentPosition <= GetSize());
	ASSERT(iSize >= 0);
	ASSERT(iSourceBuffer != NULL);

	IMediaImplementation_WriteBytes(fCurrentPosition, iSize, iSourceBuffer);
	fCurrentPosition += iSize;
	CheckThreadChangingPosition();
}

void CMedia::Write(TMediaPos iSize, const CMedia& iSourceMedia, TMediaPos iCopyBufferSize) {
	CHK_INV_SCOPE;
	ASSERT(iSize >= 0);
	ASSERT(fCurrentPosition <= GetSize());
	ASSERT(iCopyBufferSize > 0);

	std::unique_ptr<std::uint8_t> buffer(new std::uint8_t[CheckedCast<std::size_t>(iCopyBufferSize)]);
	TMediaPos sourcePosition = iSourceMedia.GetCurrentPosition();
	TMediaPos destinationPosition = GetCurrentPosition();
	TMediaPos bytesLeftToWrite = iSize;
	while (bytesLeftToWrite > 0) {
		TMediaPos chunkSize = bytesLeftToWrite;
		if (chunkSize > iCopyBufferSize) {
			chunkSize = iCopyBufferSize;
		}
		iSourceMedia.IMediaImplementation_ReadBytes(sourcePosition, chunkSize, buffer.get());
		IMediaImplementation_WriteBytes(destinationPosition, chunkSize, buffer.get());
		sourcePosition += chunkSize;
		destinationPosition += chunkSize;
		bytesLeftToWrite -= chunkSize;
	}
	iSourceMedia.SetCurrentPosition(sourcePosition);
	SetCurrentPosition(destinationPosition);
}

TMediaPos CMedia::GetCurrentPosition() const {
	CHK_INV;

	return fCurrentPosition;
}

void CMedia::SetCurrentPosition(TMediaPos iNewCurrentPosition) const {
	CHK_INV_SCOPE;

	fCurrentPosition = iNewCurrentPosition;
	CheckThreadChangingPosition();
}

void CMedia::MoveCurrentPosition(TMediaPos iRelativeAmountToMoveCurrentPosition) const {
	CHK_INV_SCOPE;

	fCurrentPosition += iRelativeAmountToMoveCurrentPosition;
	CheckThreadChangingPosition();
}

std::uint8_t CMedia::ReadByte() const {
	CHK_INV_SCOPE;

	std::uint8_t data = 0;
	Read(1, &data);
	return data;
}

std::uint8_t CMedia::PeekByte() const {
	CHK_INV_SCOPE;

	std::uint8_t data = 0;
	IMediaImplementation_ReadBytes(fCurrentPosition, 1, &data);
	return data;
}

std::uint16_t CMedia::Read16Bit() const {
	CHK_INV_SCOPE;

	std::uint16_t result = 0;
#if IS_MOTOROLA_TO_NATIVE_A_SWAP
	std::uint8_t binary[2];
	Read(2, &binary[0]);
	result = (std::uint16_t)(binary[0] << 8) | (std::uint16_t)binary[1];
#else // IS_MOTOROLA_TO_NATIVE_A_SWAP
	Read(2, &result);
#endif // IS_MOTOROLA_TO_NATIVE_A_SWAP
	return result;
}

std::uint32_t CMedia::Read32Bit() const {
	CHK_INV_SCOPE;

	std::uint32_t result = 0;
#if IS_MOTOROLA_TO_NATIVE_A_SWAP
	std::uint8_t binary[4];
	Read(4, &binary[0]);
	result = ((std::uint32_t)binary[0] << 24) | ((std::uint32_t)binary[1] << 16) | ((std::uint32_t)binary[2] << 8) | ((std::uint32_t)binary[3]);
#else // IS_MOTOROLA_TO_NATIVE_A_SWAP
	Read(4, &result);
#endif // IS_MOTOROLA_TO_NATIVE_A_SWAP
	return result;
}

double CMedia::ReadDouble() const {
	CHK_INV_SCOPE;

	std::uint8_t buffer[RSMedia::IEEE80_IMAGE_SIZE];
	Read(RSMedia::IEEE80_IMAGE_SIZE, buffer);
	double result = RSMedia::UnpackIEEE80Big(buffer);
	return result;
}

void CMedia::WriteByte(std::uint8_t iData) {
	CHK_INV_SCOPE;

	Write(1, &iData);
}

void CMedia::Write16Bit(std::uint16_t iData) {
	CHK_INV_SCOPE;

#if IS_MOTOROLA_TO_NATIVE_A_SWAP
	std::uint8_t binary[2];
	binary[0] = static_cast<std::uint8_t>(iData >> 8);
	binary[1] = static_cast<std::uint8_t>(iData & 0xff);
	Write(2, &binary[0]);
#else // IS_MOTOROLA_TO_NATIVE_A_SWAP
	Write(2, &iData);
#endif // IS_MOTOROLA_TO_NATIVE_A_SWAP
}

void CMedia::Write32Bit(std::uint32_t iData) {
	CHK_INV_SCOPE;

#if IS_MOTOROLA_TO_NATIVE_A_SWAP
	std::uint8_t binary[4];
	binary[0] = static_cast<std::uint8_t>((iData >> 24) & 0xff);
	binary[1] = static_cast<std::uint8_t>((iData >> 16) & 0xff);
	binary[2] = static_cast<std::uint8_t>((iData >> 8) & 0xff);
	binary[3] = static_cast<std::uint8_t>((iData) & 0xff);
	Write(4, &binary[0]);
#else // IS_MOTOROLA_TO_NATIVE_A_SWAP
	Write(4, &iData);
#endif // IS_MOTOROLA_TO_NATIVE_A_SWAP
}

void CMedia::WriteDouble(double iData) {
	CHK_INV_SCOPE;

	std::uint8_t buffer[RSMedia::IEEE80_IMAGE_SIZE];
	RSMedia::PackIEEE80Big(buffer, iData);
	Write(RSMedia::IEEE80_IMAGE_SIZE, buffer);
}

void CMedia::CheckThreadChangingPosition() const {
	// FL: Should not use functions depending on fCurrentPosition from multiple threads.
	//	The check is done everywhere fCurrentPosition is changed except constructors.
	//	This might still be OK, if the threads/calls are serialized,
	//	this event is just to check if it ever happens in Reason.
// ??? RZFILE FL: Disabled for now. Happens directly when CDWOPSampleDataReader
//	is used from IOWorker thread. Perhaps OK. Need to analyze more.
#if 0
	#if DEBUG
	if (fDebugPositionThreadID == RSDebug::kInvalidThreadID) {
		fDebugPositionThreadID = RSDebug::GetThisThreadID();
	}
	else if (fDebugPositionThreadID != RSDebug::GetThisThreadID()) {
		fDebugPositionThreadID = RSDebug::GetThisThreadID();
		// ### FL: Should perhaps send every event? But what if a lot of them?
		static bool haveSentEvent = false;
		if (!haveSentEvent) {
			DEV_EVENT("file_change_pos_from_mult_threads", "");
			haveSentEvent = true;
		}
	}
	#endif // DEBUG
#endif // 0
}


}	//	RSMedia
