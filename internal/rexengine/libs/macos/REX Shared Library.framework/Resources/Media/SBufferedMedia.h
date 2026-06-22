#pragma once

#include "SMedia.h"
#include "Core/Bacteria/BacteriaUtil.h"
#include <vector>

namespace RSMedia {
	 
// Wraps a CMedia and adds buffering which can speed up writes and reads when the data is accessed/modified in sequence. 
class CBufferedMedia final: public CMedia 
{
public:
	explicit CBufferedMedia(CMedia& iParentMedia, TMediaPos iBufferSize = 1024);
	explicit CBufferedMedia(const CMedia& iParentMedia, TMediaPos iBufferSize = 1024);
	~CBufferedMedia();
	RS_NO_COPY(CBufferedMedia)
	RS_NO_MOVE(CBufferedMedia)

	void IMediaImplementation_ReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iBuffer) const override;
	void IMediaImplementation_WriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iBuffer) override;
	TMediaPos IMediaImplementation_GetLength() const override;
	void Flush();

#if DEBUG
	void CheckInvariant() const override;
#endif // DEBUG

private:
	TMediaPos CurrentSize() const;
	void DoReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iBuffer) const;
	void DoWriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iBuffer);
	void InvalidateBuffer() const;
	TMediaPos BufferCapacity() const { return static_cast<TMediaPos>(fBuffer.size()); }

	enum class EBufferedDataType {
		None,
		Write,
		Read
	};

	inline static constexpr TMediaPos kUnknownMediaSize = -1;

	CMedia* fParentMedia;
	const CMedia* fParentReadOnlyMedia;
	mutable EBufferedDataType fBufferedDataType;
	mutable std::vector<std::uint8_t> fBuffer;
	mutable TMediaPos fBufferedStartPos;
	mutable TMediaPos fBufferedSize;
	mutable TMediaPos fParentMediaSize;
};

}
