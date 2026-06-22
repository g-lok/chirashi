#pragma once

#include "Core/Debug/DebugPackage.h"

#include <cstdint>

namespace RSMedia {

	
//////////////////////		CMedia


//	A media object is like an open file. COpenFile is a subclass. A media has a current position.

typedef std::int64_t TMediaPos;


// JP: CMedia subclasses implement this
class IMediaImplementation {
	public: virtual void IMediaImplementation_ReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iBuffer) const = 0;
	public: virtual void IMediaImplementation_WriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iBuffer) = 0;
	public: virtual TMediaPos IMediaImplementation_GetLength() const = 0;
};


class CMedia : public IMediaImplementation {
	protected: CMedia();	// JP: This is an abstract base class, only subclasses can be instantiated
	// only for subclasses
	protected: CMedia(const CMedia& iOther);
	public: virtual ~CMedia();
	protected: CMedia& operator=(const CMedia& iOther);

#if DEBUG
	public: virtual void CheckInvariant() const;
#endif // DEBUG

	public: TMediaPos GetSize() const;

	// Reading/writing data
	public: void Read(TMediaPos iSourcePosition, TMediaPos iSize, void* iDestinationBuffer) const;	// Reads and changes the current position
	public: void Peek(TMediaPos iSourcePosition, TMediaPos iSize, void* iDestinationBuffer) const;	// Reads but do not change the current position
	public: void Read(TMediaPos iSize, void* iDestinationBuffer) const;	// Reads and changes the current position
	public: void Write(TMediaPos iDestinationPosition, TMediaPos iSize, const void* iSourceBuffer);	// Writes and changes the current position
	public: void Write(TMediaPos iSize, const void* iSourceBuffer);	// Writes and changes the current position
	public: void Write(TMediaPos iSize, const CMedia& iSourceMedia, TMediaPos iCopyBufferSize = 8192);	// This will move the current position of both source and destination! The data will be written at the current pos of the dest media

	// Current position
	public: TMediaPos GetCurrentPosition() const;
	public: void SetCurrentPosition(TMediaPos iNewCurrentPosition) const;
	public: void MoveCurrentPosition(TMediaPos iRelativeAmountToMoveCurrentPosition) const;

	// Utilitites to read and write various primitives. Evertyhing is read/written as big endian
	public: std::uint8_t ReadByte() const;
	public: std::uint8_t PeekByte() const;
	public: std::uint16_t Read16Bit() const;
	public: std::uint32_t Read32Bit() const;
	public: double ReadDouble() const;
	public: void WriteByte(std::uint8_t iData);
	public: void Write16Bit(std::uint16_t iData);
	public: void Write32Bit(std::uint32_t iData);
	public: void WriteDouble(double iData);


	// data
	// JP: Protected so that subclasses can use this in their CheckInvariant implementations
	protected: mutable TMediaPos fCurrentPosition;

	private: void CheckThreadChangingPosition() const;
#if DEBUG
	private: mutable RSDebug::TThreadID fDebugPositionThreadID = RSDebug::kInvalidThreadID;
#endif // DEBUG
};


class CABCChildMedia : public CMedia {
	public: virtual void CABCChildMedia_PropagateCurrentPositionToParentMedia(bool iRecursive = false) const = 0;
};

}	//	RSMedia

