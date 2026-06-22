#pragma once

#include "SMedia.h"

namespace RSMedia {




/////////////////			CMemoryMedia




//	This is a media that contains a memory buffer with the data.
//	###	MZ: depricated! Use COwnedMemoryMedia or CWrappedMemoryMedia instead!


class CMemoryMedia : public CMedia {
	//	Use this to read another media into a memory media. May speed up file scanning etc.
	//	The new memory media will contain a copy of the data. This copy is owned by the new media.
	//	Writing after the allocated buffer will grow the buffer.
	//	Read/write. Write operations are allowed!
	//	The new media is positioned at pos 0.
	//	The old medias position is not changed.
	//	The entire source media is copied - the source medias position is not used!
	public: explicit CMemoryMedia(const CMedia& media);
	public: explicit CMemoryMedia(const CMemoryMedia& media);

	//	Allocates a memory media of a specific size. The media can be read/written.
	//	Writing after the allocated buffer will grow the buffer.
	//	Read/write. Write operations are allowed!
	//	The media contains garbage bytes.
	//	The new media is positioned at pos 0.
	//	The new media has size=0, but a preallocated internal buffer of TMediaPos preAllocatedSize.
	public: explicit CMemoryMedia(TMediaPos preAllocatedSize);

	//	Warning! The buffer will be referenced by the CMemoryMedia until the CMemoryMedia is deleted!
	//	Read/write. Write operations are allowed!
	//	The new media is positioned at pos 0.
	public: CMemoryMedia(TMediaPos size, std::uint8_t buffer[]);

	//	Warning! The buffer will be referenced by the CMemoryMedia until the CMemoryMedia is deleted!
	//	Read-only. No write operations are allowed!
	//	The new media is positioned at pos 0.
	public: CMemoryMedia(TMediaPos size, const std::uint8_t buffer[]);

	public: virtual ~CMemoryMedia();

#if DEBUG
	public: virtual void CheckInvariant() const;
#endif

	//	Don't use if you don't have to, please.
	//	When you write the the CMemoryMedia it may grow its buffer and the
	//	buffer returned earlier from GetBufferPtr() is then stale.
	public: const std::uint8_t* GetBufferPtr() const;

	/////////////////////		Internal stuff.

	// IMediaImplementation
	public: virtual void IMediaImplementation_ReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iDestinationBuffer) const;
	public: virtual void IMediaImplementation_WriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iSourceBuffer);
	public: virtual TMediaPos IMediaImplementation_GetLength() const;

	//	Not for application writer!
	//	This will make sure enough memory is allocated internally, counted from the media's pos.
	//	It will *not* change the medias pos. You need to SetPos() afterwards
	//	if you want this.
	public: std::uint8_t* WriteDirect(TMediaPos byteToWrite);

	private: void GrowBufferIfNeeded(TMediaPos endPos);

	// No assignment
	private: CMemoryMedia& operator=(const CMemoryMedia& other);


	//////////////////////		Data.
	private: enum EMode {
		kOwnsBufferRW	=	69,
		kReferencesBufferRW,	//	Read/write
		kReferencesBufferRO	//	Read-only.
	};
	private: EMode fMode;
	private: std::uint8_t* fBuffer;
	private: TMediaPos fAllocatedBufferSize;

	//	Only valid when mode=kOwnsBufferRW.
	private: TMediaPos fUsedBufferSize;
};








}	//	RSMedia

