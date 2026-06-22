#include "StdInclude.h"

#include "SPartialMedia.h"

#include "Core/Bacteria/Bacteria.h"
#include "Core/Debug/SpeculativeTraceMacros.h"




namespace RSMedia {


CPartialMedia::CPartialMedia(CMedia& iTotalMedia, TMediaPos iOffset) :
	fTotalWriteMedia(&iTotalMedia),
	fTotalReadMedia(NULL),
	fOffset(iOffset),
	fSize(-1)	// Indicates write. Can append and replace - not insert!
{
	ASSERT(iOffset <= iTotalMedia.GetSize());
	SetCurrentPosition(fTotalWriteMedia->GetCurrentPosition() - fOffset);
	ASSERT((fOffset + GetCurrentPosition()) <= fTotalWriteMedia->GetSize());
	CHK_INV;
}

CPartialMedia::CPartialMedia(const CMedia& iTotalMedia, TMediaPos iOffset, TMediaPos iSize) :
	fTotalWriteMedia(NULL),
	fTotalReadMedia(&iTotalMedia),
	fOffset(iOffset),
	fSize(iSize)
{
	ASSERT((iOffset + iSize) <= iTotalMedia.GetSize());
	SetCurrentPosition(fTotalReadMedia->GetCurrentPosition() - fOffset);
	ASSERT((fOffset + GetCurrentPosition()) <= fTotalReadMedia->GetSize());
	CHK_INV;
}

CPartialMedia::~CPartialMedia() {
	// JP: Can't do CheckInvariant here, our "parent media" might be gone already
//	CHK_INV;
	fTotalReadMedia = NULL;
	fTotalWriteMedia = NULL;
}

#if DEBUG
void CPartialMedia::CheckInvariant() const {
	if (fTotalWriteMedia != NULL) {
		ASSERT(fTotalReadMedia == NULL);
		ASSERT(fSize == -1);
	}
	else {
		ASSERT(fTotalReadMedia != NULL);
	}
	ASSERT(fCurrentPosition >= (-fOffset));
	ASSERT(fOffset >= 0);
}
#endif // DEBUG

void CPartialMedia::CABCChildMedia_PropagateCurrentPositionToParentMedia(bool iRecursive) const {
	CHK_INV_SCOPE;
	// FL: Should not use functions depending on position from multiple threads.
	//	See also CMedia::CheckThreadChangingPosition()
	#if DEBUG
	if (fDebugPropagateThreadID == RSDebug::kInvalidThreadID) {
		fDebugPropagateThreadID = RSDebug::GetThisThreadID();
	}
	else if (fDebugPropagateThreadID != RSDebug::GetThisThreadID()) {
		fDebugPropagateThreadID = RSDebug::GetThisThreadID();
		// ### FL: Should perhaps send every event? But what if a lot of them?
		static bool haveSentEvent = false;
		if (!haveSentEvent) {
			DEV_EVENT("file_propagate_pos_from_mult_threads", "");
			haveSentEvent = true;
		}
	}
	#endif // DEBUG

	TMediaPos localCurrentPosition = GetCurrentPosition();
	if (fTotalWriteMedia != NULL) {
		ASSERT(fTotalReadMedia == NULL);
		fTotalWriteMedia->SetCurrentPosition(localCurrentPosition + fOffset);
		if (iRecursive) {
			CABCChildMedia* parentMediaAsChildMedia = dynamic_cast<CABCChildMedia*>(fTotalWriteMedia);
			if (parentMediaAsChildMedia != NULL) {
				parentMediaAsChildMedia->CABCChildMedia_PropagateCurrentPositionToParentMedia(true);
			}
		}
	}
	else if (fTotalReadMedia != NULL) {
		fTotalReadMedia->SetCurrentPosition(localCurrentPosition + fOffset);
		if (iRecursive) {
			const CABCChildMedia* parentMediaAsChildMedia = dynamic_cast<const CABCChildMedia*>(fTotalReadMedia);
			if (parentMediaAsChildMedia != NULL) {
				parentMediaAsChildMedia->CABCChildMedia_PropagateCurrentPositionToParentMedia(true);
			}
		}
	}
	else {
		ASSERT(false);
	}
}

void CPartialMedia::IMediaImplementation_ReadBytes(TMediaPos iPosition, TMediaPos iLength, void* iDestinationBuffer) const {
	CHK_INV_SCOPE;
	ASSERT(iPosition >= 0);
	ASSERT(iLength >= 0);

	if (fTotalWriteMedia != NULL) {
		fTotalWriteMedia->IMediaImplementation_ReadBytes(iPosition + fOffset, iLength, iDestinationBuffer);
	}
	else {
		ASSERT(fTotalReadMedia != NULL);
		if ((iPosition + iLength) > fSize) {
			BTHROW RSBacteria::XRead("");
		}
		fTotalReadMedia->IMediaImplementation_ReadBytes(iPosition + fOffset, iLength, iDestinationBuffer);
	}
}

void CPartialMedia::IMediaImplementation_WriteBytes(TMediaPos iPosition, TMediaPos iLength, const void* iSourceBuffer) {
	CHK_INV_SCOPE;
	ASSERT(iPosition >=0);
	ASSERT(iLength >=0);
	if (fTotalWriteMedia == NULL) {
		BTHROW RSBacteria::XIllegalCallSequence("");
	}

	fTotalWriteMedia->IMediaImplementation_WriteBytes(iPosition + fOffset, iLength, iSourceBuffer);
}

TMediaPos CPartialMedia::IMediaImplementation_GetLength() const {
	CHK_INV_SCOPE;
	if (fSize == -1) {
		if (fTotalWriteMedia != NULL) {
			return (fTotalWriteMedia->IMediaImplementation_GetLength() - fOffset);
		}
		else {
			ASSERT(fTotalReadMedia != NULL);
			return (fTotalReadMedia->IMediaImplementation_GetLength() - fOffset);
		}
	}
	else {
		return fSize;
	}
}



}	//	RSMedia
