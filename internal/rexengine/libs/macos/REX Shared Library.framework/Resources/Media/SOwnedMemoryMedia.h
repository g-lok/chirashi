#pragma once

#include "SMedia.h"

#include <cstdint>
#include <vector>

namespace RSMedia {

 
	  
/////////////////			COwnedMemoryMedia




//	This is a media that contains its own memory buffer with the data which can grow.

class COwnedMemoryMedia : public CMedia {
	//	Use this to read another media into a memory media. May speed up file scanning etc.
	//	The new memory media will contain a copy of the data. This copy is owned by the new media.
	//	Writing after the allocated buffer will grow the buffer.
	//	Read/write. Write operations are allowed!
	//	The new media is positioned at same position as original.
	//	The old medias position is not changed.
	//	The entire source media is copied - the source medias position is not used!
	public: explicit COwnedMemoryMedia(const CMedia& iMedia);
	public: explicit COwnedMemoryMedia(const COwnedMemoryMedia& iMedia);

	//	Allocates a memory media of a specific size. The media can be read/written.
	//	Writing after the allocated buffer will grow the buffer.
	//	Read/write. Write operations are allowed!
	//	The media contains garbage bytes.
	//	The new media is positioned at pos 0.
	//	The new media has size=0, but a preallocated internal buffer of TMediaPos preAllocatedSize.
	public: explicit COwnedMemoryMedia(TMediaPos iPreAllocatedSize);

	//	Allocates a memory media of zero size. The media can be read/written.
	//	Writing after the allocated buffer will grow the buffer.
	//	Read/write. Write operations are allowed!
	//	The new media is positioned at pos 0.
	//	The new media has size=0, and a preallocated internal buffer of 0.
	public: explicit COwnedMemoryMedia();

	//	Use this to read a number of bytes into a memory media.
	//	The new memory media will contain a copy of the data. This copy is owned by the new media.
	//	Writing after the allocated buffer will grow the buffer.
	//	Read/write. Write operations are allowed!
	//	The new media is positioned at pos 0.
	public: COwnedMemoryMedia(TMediaPos iBufferSize, const std::uint8_t iBuffer[]);

	public: virtual ~COwnedMemoryMedia();

#if DEBUG
	public: virtual void CheckInvariant() const;
#endif // DEBUG

	//	Don't use if you don't have to, please.
	//	When you write the the COwnedMemoryMedia it may grow its buffer and the
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
	public: std::uint8_t* WriteDirect(TMediaPos iBytesToWrite);

	private: void GrowBufferIfNeeded(TMediaPos iEndPos);


	//////////////////////		Data.
	private: std::vector<std::uint8_t> fBuffer;



		private: COwnedMemoryMedia& operator=(const COwnedMemoryMedia&);

};


}	//	RSMedia

