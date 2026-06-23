#ifndef REX_H_
#define REX_H_

/*
	REX by Reason Studios, (C) Copyright Reason Studios AB.
	REX is a trademark of Reason Studios AB
*/

#if defined(_MSC_VER) || defined(__GNUC__)
	#pragma pack(push,8)
#else
	#error "REX not compiling with non-supported compiler."
#endif

#ifndef REX_WINDOWS
#if WINDOWS
#define REX_WINDOWS WINDOWS
#else
#define REX_WINDOWS 0
#endif
#endif // #ifndef REX_WINDOWS
#ifndef REX_MAC
#if MAC
#define REX_MAC MAC
#else
#define REX_MAC 0
#endif
#endif // #ifndef REX_MAC

#if ((!defined(REX_MAC)) || (!defined(REX_WINDOWS) ) ) || ((!(REX_MAC)) && (!(REX_WINDOWS))) || ((REX_MAC) && (REX_WINDOWS))
#error REX_MAC and REX_WINDOWS must be defined. One must be 1 and the other 0.
#endif

// Internal use only. Don't define this.
#ifndef REX_DLL_BUILD
#define REX_DLL_BUILD 1
#endif

/* Set up 'C' calling conventions */

// JE: This used to be defined as _cdecl on 32 bit REX for Windows.
#define REXCALL

/*
	Leave REX_DLL_LOADER == 1 and use REX.c to load the REX DLL / dylib bundle dynamically from a specified directory path.
	This is the default and recommended usage on Windows, especially from a plugin host or plugin module.
	Don't hard code the location - calculate it relative to some other known file, or derive the location from your installation somehow.
	The test app example shows how to calculate it relative to the location of the running executable module (.exe, .dll)

	You can set REX_DLL_LOADER = 0 globally to turn off the dependency on REX.c.

	On Mac, prefer REX_DLL_LOADER == 0, and just add a dependency on the REX .framework, and embed it in your app or plugin bundle.
	Don't forget to embed the framework - a plugin must never rely on / try and load REX from the plugin host or vice versa.
	
	If you want to use REX_DLL_LOADER == 1 on Mac (to load the dylib .bundle file manually), please make sure you can calculate the path
	you try to load it from in a general and reliable way.

	On Windows, if REX_DLL_LOADER == 0 you must use your own loader code instead of REX.c.

	It's also possible to link the included default generated link library loader .lib or framework .bundle loader
	(as recommended for REX SDK 1.8-1.9.1), however this is not supported in a shipping product, especially not for a plugin host or plugin,
	because it can break the library for itself, any other plugins with the same issue and the host.
*/
#define REX_ABI 
#ifndef REX_DLL_LOADER
#define REX_DLL_LOADER REX_WINDOWS
#endif

#if (REX_DLL_LOADER) && (REX_WINDOWS)
#include <wchar.h>
#endif

#ifdef __cplusplus
namespace REX {
	extern "C" {
#endif



#if !REX_TYPES_DEFINED
#if defined(_MSC_VER)
/* Safe guess on MSC */
typedef __int32 REX_int32_t;
#elif defined(__GNUC__)
/* GCC on any platform (Mac, Windows/MinGW) */
typedef int REX_int32_t;
#else
#error "REX: REX_int32_t not defined."
#endif /* defined(_MSC_VER) */
#endif /* !REX_TYPES_DEFINED */


#define kREXPPQ 15360
#define kREXStringSize 255


typedef enum {
	/* Not really errors */
	kREXError_NoError					= 1,
	kREXError_OperationAbortedByUser	= 2,
	kREXError_NoCreatorInfoAvailable	= 3,

	/* Run-time errors */
#if REX_DLL_LOADER
	kREXError_NotEnoughMemoryForDLL		= 100,
	kREXError_UnableToLoadDLL			= 101,
	kREXError_DLLTooOld					= 102,
	kREXError_DLLNotFound				= 103,
	kREXError_APITooOld					= 104,
#endif // #if REX_DLL_LOADER
	kREXError_OutOfMemory				= 105,
	kREXError_FileCorrupt				= 106,
	kREXError_REX2FileTooNew			= 107,
	kREXError_FileHasZeroLoopLength		= 108,
#if REX_DLL_LOADER
	kREXError_OSVersionNotSupported		= 109,
#endif

	/* Implementation errors */
	kREXImplError_DLLNotInitialized		= 200,
	kREXImplError_DLLAlreadyInitialized	= 201,
#if REX_DLL_LOADER
	kREXImplError_DLLNotLoaded			= kREXImplError_DLLNotInitialized,
	kREXImplError_DLLAlreadyLoaded		= kREXImplError_DLLAlreadyInitialized,
#endif
	kREXImplError_InvalidHandle			= 202,
	kREXImplError_InvalidSize			= 203,
	kREXImplError_InvalidArgument		= 204,
	kREXImplError_InvalidSlice			= 205,
	kREXImplError_InvalidSampleRate		= 206,
	kREXImplError_BufferTooSmall		= 207,
	kREXImplError_IsBeingPreviewed		= 208,
	kREXImplError_NotBeingPreviewed		= 209,
	kREXImplError_InvalidTempo			= 210,

	/* DLL error - call the cops! */
	kREXError_Undefined					= 666
} REXError;

typedef void* REXHandle;

typedef struct {
	REX_int32_t fChannels;
	REX_int32_t fSampleRate;
	REX_int32_t fSliceCount;		/* Number of slices */
	REX_int32_t fTempo;				/* Tempo set when exported from ReCycle, 123.456 BPM stored as 123456L etc. */
	REX_int32_t fOriginalTempo;		/* Original tempo of loop, as calculated by ReCycle from the locator positions and bars/beats/sign settings. */
	REX_int32_t fPPQLength;			/* Length of loop */
	REX_int32_t fTimeSignNom;
	REX_int32_t fTimeSignDenom;
	REX_int32_t fBitDepth;			/* Number of bits per sample in original data */
} REXInfo;


typedef struct {
	REX_int32_t fPPQPos;			/* Position of slice in loop */
	REX_int32_t fSampleLength;		/* Length of rendered slice, at its original sample rate. */
} REXSliceInfo;


typedef struct {
	char fName[kREXStringSize + 1];
	char fCopyright[kREXStringSize + 1];
	char fURL[kREXStringSize + 1];
	char fEmail[kREXStringSize + 1];
	char fFreeText[kREXStringSize + 1];
} REXCreatorInfo;


typedef enum {
	kREXCallback_Abort = 1,
	kREXCallback_Continue = 2
} REXCallbackResult;


typedef REXCallbackResult (REXCALL *REXCreateCallback)(REX_int32_t percentFinished, void* userData);



#if REX_DLL_LOADER
// iDirPath is the absolute path to the directory containing REX DLL / framework-bundle
#if REX_WINDOWS
REXError REXInitializeDLL_DirPath(const wchar_t* iDirPath);
#else
REXError REXInitializeDLL_DirPath(const char* iDirPathUTF8);
#endif
#else
REX_ABI REXError REXInitializeDLL(void);
#endif

REX_ABI void REXUninitializeDLL(void);

REX_ABI REXError REXCreate(REXHandle* handle, 
	const char buffer[], 
	REX_int32_t size,
	REXCreateCallback callbackFunc,
	void* userData);

REX_ABI void REXDelete(REXHandle* handle);

REX_ABI REXError REXGetInfo(REXHandle handle,
	REX_int32_t infoSize,
	REXInfo* info);

REX_ABI REXError REXGetInfoFromBuffer(
	REX_int32_t bufferSize,
	const char buffer[],
	REX_int32_t infoSize,
	REXInfo* info);

REX_ABI REXError REXGetCreatorInfo(REXHandle handle,
	REX_int32_t creatorInfoSize,
	REXCreatorInfo* creatorInfo);

REX_ABI REXError REXGetSliceInfo(REXHandle handle, 
	REX_int32_t sliceIndex, 
	REX_int32_t sliceInfoSize,
	REXSliceInfo* sliceInfo);

REX_ABI REXError REXSetOutputSampleRate(REXHandle handle,
	REX_int32_t outputSampleRate);

REX_ABI REXError REXRenderSlice(REXHandle handle, 
	REX_int32_t sliceIndex, 
	REX_int32_t bufferFrameLength,
	float* outputBuffers[2]);

REX_ABI REXError REXStartPreview(REXHandle handle);

REX_ABI REXError REXStopPreview(REXHandle handle);

REX_ABI REXError REXRenderPreviewBatch(REXHandle handle, 
	REX_int32_t framesToRender, 
	float* outputBuffers[2]);

REX_ABI REXError REXSetPreviewTempo(REXHandle handle, 
	REX_int32_t tempo);




#ifdef __cplusplus
	}
}
#endif

#if defined(_MSC_VER) || defined(__GNUC__)
	#pragma pack(pop)
#else
	#error "REX not compiling with non-supported compiler"
#endif

#endif /* REX_H_ */
