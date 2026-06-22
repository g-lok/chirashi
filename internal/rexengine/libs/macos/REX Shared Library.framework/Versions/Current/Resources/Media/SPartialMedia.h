#pragma once

#include "SMedia.h"

namespace RSMedia {


class CPartialMedia : public CABCChildMedia {
	public: CPartialMedia(const CMedia& iTotalMedia,TMediaPos iOffset, TMediaPos iSize);
	public: CPartialMedia(CMedia& iTotalMedia, TMediaPos iOffset);
	public: virtual ~CPartialMedia();

#if DEBUG
		public: virtual void CheckInvariant() const;
#endif // DEBUG

	// CABCChildMedia
	public: virtual void CABCChildMedia_PropagateCurrentPositionToParentMedia(bool iRecursive = false) const;

	/////////////	Internal stuff
	// IMediaImplementation
	public: virtual void IMediaImplementation_ReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iDestinationBuffer) const;
	public: virtual void IMediaImplementation_WriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iSourceBuffer);
	public: virtual TMediaPos IMediaImplementation_GetLength() const;

	// Not supported 
	private: CPartialMedia();
	private: CPartialMedia(const CPartialMedia& other);
	private: CPartialMedia& operator= (const CPartialMedia& other);

	// Data
	private: CMedia* fTotalWriteMedia;
	private: const CMedia* fTotalReadMedia;
	private: TMediaPos fOffset;
	private: TMediaPos fSize;

#if DEBUG
	private: mutable RSDebug::TThreadID fDebugPropagateThreadID = RSDebug::kInvalidThreadID;
#endif // DEBUG
};


}	//	RSMedia

