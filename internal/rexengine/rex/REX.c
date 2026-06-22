
/*
	REX by Reason Studios, (C) Copyright Reason Studios AB.
	REX is a trademark of Reason Studios AB

	REX.c - dynamic loader
*/

#include "REX.h"
#if !(REX_DLL_LOADER)
#error REX_DLL_LOADER should be 1 for projects using REX.c
#endif

/* Internal stuff */

/*	Format is aaabbbccc in decimal. aaa=major,bbb=minor,ccc=revision. That
	is: versionLong=major * 1000000 + minor * 1000 + revision.
*/

#define REX_BUILD_VERSION(major,minor,revision)	((major) * 1000000 + (minor) * 1000 + (revision))
#define REX_API_VERSION		REX_BUILD_VERSION(1,1,1)
#define REX_FIRST_64BIT_VERSION	REX_BUILD_VERSION(1,7,0)

/*
 *
 * Local assertion macro.
 *
 */
#include <assert.h>
#define REX_ASSERT(e) assert(e)

#if REX_WINDOWS
	#define NOGDICAPMASKS
	#define NOVIRTUALKEYCODES
	#define NOWINSTYLES
	#define NOSYSMETRICS
	#define NOMENUS
	#define NOICONS
	#define NOKEYSTATES
	#define NOSYSCOMMANDS
	#define NORASTEROPS
	#define NOSHOWWINDOW
	#define OEMRESOURCE
	#define NOATOM
	#define NOCLIPBOARD
	#define NOCOLOR
	#define NODRAWTEXT
	#define NONLS
	#define NOMB
	#define NOMEMMGR
	#define NOMETAFILE
	#define NOMINMAX
	#define NOOPENFILE
	#define NOSCROLL
	#define NOSERVICE
	#define NOSOUND
	#define NOTEXTMETRIC
	#define NOWH
	#define NOWINOFFSETS
	#define NOCOMM
	#define NOKANJI
	#define NOHELP
	#define NOPROFILER
	#define NODEFERWINDOWPOS
	#define NOMCX
	#define WIN32_LEAN_AND_MEAN
	#include <windows.h>
	#include <stdlib.h>
	#include <shlobj.h>
	#include <VersionHelpers.h>
#endif /* REX_WINDOWS */
#if REX_MAC
#include <CoreFoundation/CFBundle.h>
#include <string.h>
#include <sys/errno.h>
#include <sys/syslimits.h>
#ifndef TRUE
#define TRUE	1
#define FALSE	0
#endif /* ndef TRUE */
#endif /* REX_MAC */


#ifdef __cplusplus
namespace REX {
#endif /* def __cplusplus */


static const unsigned short kREXCompatibleMajor = (short)(REX_API_VERSION / 1000000L);


static REX_int32_t gREXLoadCount = 0;

#if REX_WINDOWS
static HINSTANCE gREXDLLInstance = 0;
static const WCHAR kWinREXDebugDLLPathName[] = L"REX Shared Library Debugging.dll";
static const WCHAR kWinREXBetaDLLPathName[] = L"REX Shared Library Testing.dll";
static const WCHAR kWinREXReleaseDLLPathName[] = L"REX Shared Library.dll";
#endif /* REX_WINDOWS */

#if REX_MAC
static CFBundleRef gREXMachODLLInstance = NULL;
static const char kMacREXDebuggingMachODLLFileName[] = "REX Shared Library Debugging.bundle";
static const char kMacREXTestingMachODLLFileName[] = "REX Shared Library Testing.bundle";
static const char kMacREXDeploymentMachODLLFileName[] = "REX Shared Library.bundle";
#endif /* REX_MAC */




#if REX_WINDOWS
typedef int (_cdecl *REXPROC)();
#endif
#if REX_MAC
typedef int (*REXPROC)(void);
#endif





/*
	Win32 platform code
*/

#if REX_WINDOWS

static char WinIsThisWindowsVersionSupported()
{
	return IsWindows7OrGreater();
}

static char WinGetFileVersion(LPCWSTR pathName,short* outRevision, short* outVersion, short* outBuildHi, short* outBuildLo){
	DWORD lpdwHandle=0;
	DWORD lSize;
	UINT lOldErrorMode;
	char result=FALSE;

	lOldErrorMode=SetErrorMode(SEM_NOOPENFILEERRORBOX);

	lSize=GetFileVersionInfoSizeW(pathName,&lpdwHandle);
	if(0!=lSize){
		BOOL resultFlag;
		char* buffer=0;
		buffer=(char*)malloc(lSize);
		if(0!=buffer){
			resultFlag=GetFileVersionInfoW(pathName, 0, lSize, buffer);
			if(0!=resultFlag){
				VS_FIXEDFILEINFO* lFInfo=0;
				UINT lFLength;

				resultFlag=VerQueryValueW(buffer, L"\\", &lFInfo, &lFLength);
				if(0!=resultFlag){
					*outRevision=HIWORD(lFInfo->dwFileVersionMS);
					*outVersion=LOWORD(lFInfo->dwFileVersionMS);
					*outBuildHi=HIWORD(lFInfo->dwFileVersionLS);
					*outBuildLo=LOWORD(lFInfo->dwFileVersionLS);
					result=TRUE;
				}
			}
			free(buffer);
			buffer=0;
		}
	}
	SetErrorMode(lOldErrorMode);
	return result;
}

static REXError VerifyREXDLLVersion(){
	WCHAR completeFileName[256 + 1];
	DWORD result = GetModuleFileNameW(gREXDLLInstance, completeFileName, 256);
	if(result == 0){
		return kREXError_UnableToLoadDLL;
	}
	{
		short revision;
		short version;
		short buildHi;
		short buildLo;
		WinGetFileVersion(completeFileName, &revision, &version, &buildHi, &buildLo);
		if(revision != kREXCompatibleMajor){
			if (revision > kREXCompatibleMajor) {
				return kREXError_APITooOld;
			} else {
				return kREXError_DLLTooOld;
			}
		}
		{
			REX_int32_t apiVersion = REX_API_VERSION;
			REX_int32_t gotVersion = REX_BUILD_VERSION(revision, version, buildHi);
			if (apiVersion > gotVersion) {
				return kREXError_DLLTooOld;
			}
			if (REX_FIRST_64BIT_VERSION > gotVersion) { 
				return kREXError_DLLTooOld;
			}
		}
	}
	return kREXError_NoError;
}

static REXError AppendToPathSafely(LPWSTR ioCurrentPath, LPCWSTR iPathToAppend){
	size_t currentLength = wcslen(ioCurrentPath);
	size_t appendLength = wcslen(iPathToAppend);
	if(currentLength + appendLength + 1 < MAX_PATH){
		if ( (currentLength != 0) &&
			 (ioCurrentPath[0] != L'\\') )
		{
			wcscpy(&ioCurrentPath[currentLength], L"\\");
			++currentLength;
		}
		wcscpy(&ioCurrentPath[currentLength], iPathToAppend);
		return kREXError_NoError;
	}
	else{
		return kREXError_OutOfMemory;
	}
}

static REXError LoadREXDLLFromPath(LPCWSTR path){
	REXError rErrorResult = kREXError_UnableToLoadDLL;

	UINT oldErrorMode = SetErrorMode(SEM_FAILCRITICALERRORS);

	gREXDLLInstance=LoadLibraryW(path);
	if(NULL==gREXDLLInstance){
		DWORD error=GetLastError();
		switch(error){
			case ERROR_NOT_ENOUGH_MEMORY:
			case ERROR_OUTOFMEMORY:
			case ERROR_TOO_MANY_OPEN_FILES:
				rErrorResult = kREXError_NotEnoughMemoryForDLL;
				break;
			case ERROR_MOD_NOT_FOUND:
				rErrorResult = kREXError_DLLNotFound;
				break;
			default:
				rErrorResult = kREXError_UnableToLoadDLL;
				break;
		}
	}else{
		rErrorResult = kREXError_NoError;
	}
	SetErrorMode(oldErrorMode);
	return rErrorResult;
}


static REXError LoadREXDLLByName(LPCWSTR dllName, LPCWSTR iDirPath){
	if (iDirPath != NULL) {
		WCHAR dllPath[MAX_PATH + 1];
		wcscpy(dllPath, iDirPath);
		if(AppendToPathSafely(dllPath, dllName) == kREXError_NoError){
			if(LoadREXDLLFromPath(dllPath) == kREXError_NoError){
				return kREXError_NoError;
			}
		}
	}
	return LoadREXDLLFromPath(dllName);
}

static REXError LoadREXDLL(const wchar_t* iDirPath){
	REX_ASSERT(iDirPath != NULL);
	if (FALSE==WinIsThisWindowsVersionSupported()) {
		return kREXError_OSVersionNotSupported;
	}

	REXError result=LoadREXDLLByName(kWinREXDebugDLLPathName, iDirPath);
	if(result!=kREXError_NoError){
		result=LoadREXDLLByName(kWinREXBetaDLLPathName, iDirPath);
	}
	if(result!=kREXError_NoError){
		result=LoadREXDLLByName(kWinREXReleaseDLLPathName, iDirPath);
	}

	return result;
}

static REXPROC FindREXDLLFunction(const char functionName[]){
	REXPROC result=(REXPROC)GetProcAddress(gREXDLLInstance,functionName);
	return result;
}

static void UnloadREXDLL(){
	if(0!=gREXDLLInstance){
		FreeLibrary(gREXDLLInstance);
		gREXDLLInstance=0;
	}
}

static char IsREXLoadedByThisInstance(){
	if(0==gREXDLLInstance){
		return FALSE;
	}
	return TRUE;
}

#endif /* REX_WINDOWS */







/*
	MacOS platform code
*/

#if REX_MAC

static REXError VerifyREXDLLVersion() {
	REXError result = kREXError_UnableToLoadDLL;
	UInt32 packedVersion;
	NumVersion* versionPtr;

	if (gREXMachODLLInstance != NULL) {
		packedVersion = CFBundleGetVersionNumber(gREXMachODLLInstance);
		versionPtr = (NumVersion*)(&packedVersion);
		UInt8 major = versionPtr->majorRev;
		UInt8 minor = (versionPtr->minorAndBugRev & 0x00f0) >> 4;
		UInt8 revision = (versionPtr->minorAndBugRev & 0x000f);
		REX_int32_t gotVersion = REX_BUILD_VERSION(major, minor, revision);

		if (major < kREXCompatibleMajor) {
			result = kREXError_DLLTooOld;
		}
		else if (major > kREXCompatibleMajor) {
			result = kREXError_APITooOld;
		}
		else if(REX_API_VERSION > gotVersion) {
			result = kREXError_DLLTooOld;
		}
		else if(REX_FIRST_64BIT_VERSION > gotVersion) {
			result = kREXError_DLLTooOld;
		}
		else {
			result = kREXError_NoError;
		}
	}
	else {
		REX_ASSERT(FALSE);
	}
	return result;
}

static char MacGetMacURLRefForREXDLLByName(const char* iDirPathUTF8, const char dllName[], CFURLRef* outFoundURL) {
	REX_ASSERT(iDirPathUTF8 != NULL);
	REX_ASSERT(outFoundURL != NULL);
	REX_ASSERT(dllName != NULL);

	CFStringRef pathCFString = CFStringCreateWithCString(kCFAllocatorDefault, iDirPathUTF8, kCFStringEncodingUTF8);
	CFStringRef nameCFString = CFStringCreateWithCString(kCFAllocatorDefault, dllName, kCFStringEncodingUTF8);
	CFURLRef dirURL = CFURLCreateWithFileSystemPath(kCFAllocatorDefault, pathCFString, kCFURLPOSIXPathStyle, TRUE);
	CFRelease(pathCFString);
	CFURLRef bundleURL = CFURLCreateWithFileSystemPathRelativeToBase(kCFAllocatorDefault, nameCFString, kCFURLPOSIXPathStyle, FALSE, dirURL);
	CFRelease(nameCFString);
	CFRelease(dirURL);

	CFBundleRef bundle = CFBundleCreate(kCFAllocatorDefault, bundleURL);

	if (bundle != NULL) {
		CFRelease(bundle);
		*outFoundURL = bundleURL;
		return TRUE;
	}

	CFRelease(bundleURL);
	return FALSE;
}

static char MacGetURLRefForREXDLL(const char* iDirPathUTF8, CFURLRef* outFoundURL) {
	REX_ASSERT(outFoundURL != NULL);

	if (MacGetMacURLRefForREXDLLByName(iDirPathUTF8, kMacREXDebuggingMachODLLFileName, outFoundURL)) {
		REX_ASSERT(*outFoundURL != NULL);
		return TRUE;
	}
	if (MacGetMacURLRefForREXDLLByName(iDirPathUTF8, kMacREXTestingMachODLLFileName, outFoundURL)) {
		REX_ASSERT(*outFoundURL != NULL);
		return TRUE;
	}
	if (MacGetMacURLRefForREXDLLByName(iDirPathUTF8, kMacREXDeploymentMachODLLFileName, outFoundURL)) {
		REX_ASSERT(*outFoundURL != NULL);
		return TRUE;
	}
	return FALSE;
}

static REXError LoadREXMachODLL(const char* iDirPath) {
	REXError result = kREXError_UnableToLoadDLL;
	CFURLRef macFileURL = NULL;

	REX_ASSERT(gREXMachODLLInstance == NULL);

	if (MacGetURLRefForREXDLL(iDirPath, &macFileURL)) {
		REX_ASSERT(macFileURL != NULL);
		gREXMachODLLInstance = CFBundleCreate(kCFAllocatorDefault, macFileURL);
		CFRelease(macFileURL);
		macFileURL = NULL;
		if (gREXMachODLLInstance == NULL) {
			result = kREXError_UnableToLoadDLL;
		}
		else {
			result = kREXError_NoError;
		}
	}
	return result;
}

static REXError LoadREXDLL(const char* iDirPath) {
	REXError result = kREXError_UnableToLoadDLL;

	REX_ASSERT(gREXMachODLLInstance == NULL);

	result = LoadREXMachODLL(iDirPath);
	return result;
}

static REXPROC FindREXDLLFunction(const char functionName[]) {
	CFStringRef functionNameStringRef;
	REXPROC result;

	REX_ASSERT(functionName != NULL);

	if (gREXMachODLLInstance != NULL) {
		functionNameStringRef = CFStringCreateWithCString(kCFAllocatorDefault, functionName, kCFStringEncodingUTF8);
		if (functionNameStringRef == NULL) {
			REX_ASSERT(FALSE);
			return NULL;
		}
		REX_ASSERT(sizeof(unsigned long) >= sizeof(void*));
		result = (REXPROC)((unsigned long)CFBundleGetFunctionPointerForName(gREXMachODLLInstance, functionNameStringRef));
		CFRelease(functionNameStringRef);
		functionNameStringRef = NULL;
		REX_ASSERT(result != NULL);
		return result;
	}
	return NULL;
}

static void UnloadREXDLL() {
	if (gREXMachODLLInstance != NULL) {
		/*
			JP: Work-around for nasty problem with GCC 3.3: if any exceptions has been thrown (and caught) inside the REX library,
			the application will crash on exit if we release the REX bundle here.
			Therefore, only CFRelease the bundle on x86 (where the REX library is built using GCC 4).

			JE: Keeping old comment above for historical context. It explains why the CFRelease() below was #if for 32 bit build only.
 			We are a long way from GCC 3/4 times now, and it seems we should call CFRelease() here. But we don't want to crash on exit.
		*/
		//CFRelease(gREXMachODLLInstance);
		gREXMachODLLInstance = NULL;
	}
	else {
		REX_ASSERT(FALSE);
	}
}

static char IsREXLoadedByThisInstance(){
	if ((gREXMachODLLInstance != NULL)) {
		return TRUE;
	}
	return FALSE;
}

#endif /* REX_MAC */

static REXPROC FindREXDLLFunction_Fallback(const char functionName1[], const char functionName2[]){
	REXPROC result=FindREXDLLFunction(functionName1);
	if (result == NULL) {
		result=FindREXDLLFunction(functionName2);
	}
	return result;
}








/* 
	To support a new platform these functions need to be implemented:

		REXError LoadREXDLL();
		char IsREXLoadedByThisInstance();
		REXError VerifyREXDLLVersion();
		REXPROC FindREXDLLFunction(const char functionName[]){
		void UnloadREXDLL();
*/


/*
	Portable code
*/


typedef char (REXCALL *TDLLOpenProc)(void);
typedef void (REXCALL *TDLLCloseProc)(void);
typedef REXError (REXCALL *TREXCreateProc)(REXHandle*, const char [], REX_int32_t, REXCreateCallback, void*);
typedef void (REXCALL *TREXDeleteProc)(REXHandle*);
typedef REXError (REXCALL *TREXGetInfoProc)(REXHandle, REX_int32_t, REXInfo*);
typedef REXError (REXCALL *TREXGetInfoFromBufferProc)(REX_int32_t, const char [], REX_int32_t, REXInfo*);
typedef REXError (REXCALL *TREXGetCreatorInfoProc)(REXHandle, REX_int32_t, REXCreatorInfo*);
typedef REXError (REXCALL *TREXGetSliceInfoProc)(REXHandle, REX_int32_t, REX_int32_t, REXSliceInfo*);
typedef REXError (REXCALL *TREXSetOutputSampleRateProc)(REXHandle, REX_int32_t);
typedef REXError (REXCALL *TREXRenderSliceProc)(REXHandle, REX_int32_t, REX_int32_t, float* [2]);
typedef REXError (REXCALL *TREXStartPreviewProc)(REXHandle);
typedef REXError (REXCALL *TREXStopPreviewProc)(REXHandle);
typedef REXError (REXCALL *TREXRenderPreviewBatchProc)(REXHandle, REX_int32_t, float* [2]);
typedef REXError (REXCALL *TREXSetPreviewTempoProc)(REXHandle, REX_int32_t);

static const char kDLLOpenProcName[]="Open";
static const char kDLLCloseProcName[]="Close";
static const char kREXInitializeDLLProcName[]="REXInitializeDLL";
static const char kREXUninitializeDLLProcName[]="REXUninitializeDLL";
static const char kREXCreateProcName[]="REXCreate";
static const char kREXDeleteProcName[]="REXDelete";
static const char kREXGetInfoProcName[]="REXGetInfo";
static const char kREXGetInfoFromBufferProcName[]="REXGetInfoFromBuffer";
static const char kREXGetCreatorInfoProcName[]="REXGetCreatorInfo";
static const char kREXGetSliceInfoProcName[]="REXGetSliceInfo";
static const char kREXSetOutputSampleRateProcName[]="REXSetOutputSampleRate";
static const char kREXRenderSliceProcName[]="REXRenderSlice";
static const char kREXStartPreviewProcName[]="REXStartPreview";
static const char kREXStopPreviewProcName[]="REXStopPreview";
static const char kREXRenderPreviewBatchProcName[]="REXRenderPreviewBatch";
static const char kREXSetPreviewTempoProcName[]="REXSetPreviewTempo";

static TDLLOpenProc gDLLOpenProc = NULL;
static TDLLCloseProc gDLLCloseProc = NULL;
static TREXCreateProc gREXCreateProc = NULL;
static TREXDeleteProc gREXDeleteProc = NULL;
static TREXGetInfoProc gREXGetInfoProc = NULL;
static TREXGetInfoFromBufferProc gREXGetInfoFromBufferProc = NULL;
static TREXGetCreatorInfoProc gREXGetCreatorInfoProc = NULL;
static TREXGetSliceInfoProc gREXGetSliceInfoProc = NULL;
static TREXSetOutputSampleRateProc gREXSetOutputSampleRateProc = NULL;
static TREXRenderSliceProc gREXRenderSliceProc = NULL;
static TREXStartPreviewProc gREXStartPreviewProc = NULL;
static TREXStopPreviewProc gREXStopPreviewProc = NULL;
static TREXRenderPreviewBatchProc gREXRenderPreviewBatchProc = NULL;
static TREXSetPreviewTempoProc gREXSetPreviewTempoProc = NULL;


static void ClearProcPointers() {
	gDLLOpenProc = NULL;
	gDLLCloseProc = NULL;
	gREXCreateProc = NULL;
	gREXDeleteProc = NULL;
	gREXGetInfoProc = NULL;
	gREXGetInfoFromBufferProc = NULL;
	gREXGetCreatorInfoProc = NULL;
	gREXGetSliceInfoProc = NULL;
	gREXSetOutputSampleRateProc = NULL;
	gREXRenderSliceProc = NULL;
	gREXStartPreviewProc = NULL;
	gREXStopPreviewProc = NULL;
	gREXRenderPreviewBatchProc = NULL;
	gREXSetPreviewTempoProc = NULL;
}


REXError REXCreate(REXHandle* handle, const char buffer[], REX_int32_t size, REXCreateCallback callbackFunc, void* userData) {
	if (!IsREXLoadedByThisInstance() || (gREXCreateProc == NULL)){
		return kREXImplError_DLLNotLoaded;
	}
	else {
#if REX_WINDOWS
		return gREXCreateProc(handle, buffer, size, callbackFunc, userData);
#elif REX_MAC
		if (gREXMachODLLInstance != NULL) {
			return gREXCreateProc(handle, buffer, size, callbackFunc, userData);
		}
		/* What, gREXMachODLLInstance is not set?!? */
		return kREXImplError_DLLNotLoaded;
#endif /* REX_MAC */
	}
}

void REXDelete(REXHandle* handle)
{
	if(!IsREXLoadedByThisInstance() || gREXDeleteProc == NULL){
		REX_ASSERT(FALSE);	/* DLL has not been loaded! */
	} else {
		gREXDeleteProc(handle);
	}
}

REXError REXGetInfo(REXHandle handle, REX_int32_t infoSize, REXInfo* info)
{
	if(!IsREXLoadedByThisInstance() || gREXGetInfoProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXGetInfoProc(handle, infoSize, info);
	}
}

REXError REXGetInfoFromBuffer(REX_int32_t bufferSize, const char buffer[], REX_int32_t infoSize, REXInfo* info)
{
	if(!IsREXLoadedByThisInstance() || gREXGetInfoFromBufferProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXGetInfoFromBufferProc(bufferSize, buffer, infoSize, info);
	}
}

REXError REXGetCreatorInfo(REXHandle handle, REX_int32_t creatorInfoSize, REXCreatorInfo* creatorInfo)
{
	if(!IsREXLoadedByThisInstance() || gREXGetCreatorInfoProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXGetCreatorInfoProc(handle, creatorInfoSize, creatorInfo);
	}
}

REXError REXGetSliceInfo(REXHandle handle, REX_int32_t sliceIndex, REX_int32_t sliceInfoSize, REXSliceInfo* sliceInfo)
{
	if(!IsREXLoadedByThisInstance() || gREXGetSliceInfoProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXGetSliceInfoProc(handle, sliceIndex, sliceInfoSize, sliceInfo);
	}
}

REXError REXSetOutputSampleRate(REXHandle handle, REX_int32_t outputSampleRate)
{
	if(!IsREXLoadedByThisInstance() || gREXSetOutputSampleRateProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXSetOutputSampleRateProc(handle, outputSampleRate);
	}
}

REXError REXRenderSlice(REXHandle handle, REX_int32_t sliceIndex, REX_int32_t bufferFrameLength, float* outputBuffers[2])
{
	if(!IsREXLoadedByThisInstance() || gREXRenderSliceProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXRenderSliceProc(handle, sliceIndex, bufferFrameLength, outputBuffers);
	}
}

REXError REXStartPreview(REXHandle handle)
{
	if(!IsREXLoadedByThisInstance() || gREXStartPreviewProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXStartPreviewProc(handle);
	}
}

REXError REXStopPreview(REXHandle handle)
{
	if(!IsREXLoadedByThisInstance() || gREXStopPreviewProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXStopPreviewProc(handle);
	}
}

REXError REXRenderPreviewBatch(REXHandle handle, REX_int32_t framesToRender, float* outputBuffers[2])
{
	if(!IsREXLoadedByThisInstance() || gREXRenderPreviewBatchProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXRenderPreviewBatchProc(handle, framesToRender, outputBuffers);
	}
}

REXError REXSetPreviewTempo(REXHandle handle, REX_int32_t tempo)
{
	if(!IsREXLoadedByThisInstance() || gREXSetPreviewTempoProc == NULL){
		return kREXImplError_DLLNotLoaded;
	} else {
		return gREXSetPreviewTempoProc(handle, tempo);
	}
}




#if REX_WINDOWS
REXError REXInitializeDLL_DirPath(const wchar_t* iDirPath) {
#else
REXError REXInitializeDLL_DirPath(const char* iDirPathUTF8) {
#endif
#if REX_WINDOWS
	REX_ASSERT(iDirPath != NULL);
#else
	REX_ASSERT(iDirPathUTF8 != NULL);
#endif
	if (IsREXLoadedByThisInstance()) {
		REX_ASSERT(FALSE);		/* REXLoadDLL() should only be called once before it is unloaded by calling REXUninitializeDLL()! */
		gREXLoadCount++;
		return kREXImplError_DLLAlreadyLoaded;
	}
	else {
		REX_ASSERT(gREXLoadCount == 0);
		REX_ASSERT(gDLLOpenProc == NULL);
		REX_ASSERT(gDLLCloseProc == NULL);
		REX_ASSERT(gREXCreateProc == NULL);
		REX_ASSERT(gREXDeleteProc == NULL);
		REX_ASSERT(gREXGetInfoProc == NULL);
		REX_ASSERT(gREXGetInfoFromBufferProc == NULL);
		REX_ASSERT(gREXGetCreatorInfoProc == NULL);
		REX_ASSERT(gREXGetSliceInfoProc == NULL);
		REX_ASSERT(gREXSetOutputSampleRateProc == NULL);
		REX_ASSERT(gREXRenderSliceProc == NULL);
		REX_ASSERT(gREXStartPreviewProc == NULL);
		REX_ASSERT(gREXStopPreviewProc == NULL);
		REX_ASSERT(gREXRenderPreviewBatchProc == NULL);
		REX_ASSERT(gREXSetPreviewTempoProc == NULL);
		{
#if REX_WINDOWS
			REXError result = LoadREXDLL(iDirPath);
#else
			REXError result = LoadREXDLL(iDirPathUTF8);
#endif

			if (result == kREXError_NoError) {
				result = VerifyREXDLLVersion();
				if (result != kREXError_NoError) {
					UnloadREXDLL();
					return result;
				}
				else {
					gREXLoadCount = 1;
					
					gDLLOpenProc = (TDLLOpenProc)FindREXDLLFunction_Fallback(kREXInitializeDLLProcName, kDLLOpenProcName);
					gDLLCloseProc = (TDLLCloseProc)FindREXDLLFunction_Fallback(kREXUninitializeDLLProcName, kDLLCloseProcName);
					gREXCreateProc = (TREXCreateProc)FindREXDLLFunction(kREXCreateProcName);
					gREXDeleteProc = (TREXDeleteProc)FindREXDLLFunction(kREXDeleteProcName);
					gREXGetInfoProc = (TREXGetInfoProc)FindREXDLLFunction(kREXGetInfoProcName);
					gREXGetInfoFromBufferProc = (TREXGetInfoFromBufferProc)FindREXDLLFunction(kREXGetInfoFromBufferProcName);
					gREXGetCreatorInfoProc = (TREXGetCreatorInfoProc)FindREXDLLFunction(kREXGetCreatorInfoProcName);
					gREXGetSliceInfoProc = (TREXGetSliceInfoProc)FindREXDLLFunction(kREXGetSliceInfoProcName);
					gREXSetOutputSampleRateProc = (TREXSetOutputSampleRateProc)FindREXDLLFunction(kREXSetOutputSampleRateProcName);
					gREXRenderSliceProc = (TREXRenderSliceProc)FindREXDLLFunction(kREXRenderSliceProcName);
					gREXStartPreviewProc = (TREXStartPreviewProc)FindREXDLLFunction(kREXStartPreviewProcName);
					gREXStopPreviewProc = (TREXStopPreviewProc)FindREXDLLFunction(kREXStopPreviewProcName);
					gREXRenderPreviewBatchProc = (TREXRenderPreviewBatchProc)FindREXDLLFunction(kREXRenderPreviewBatchProcName);
					gREXSetPreviewTempoProc = (TREXSetPreviewTempoProc)FindREXDLLFunction(kREXSetPreviewTempoProcName);

					if ((gDLLOpenProc == NULL) ||
						(gDLLCloseProc == NULL) ||
						(gREXCreateProc == NULL) ||
						(gREXDeleteProc == NULL) ||
						(gREXGetInfoProc == NULL) ||
						(gREXGetInfoFromBufferProc == NULL) ||
						(gREXGetCreatorInfoProc == NULL) ||
						(gREXGetSliceInfoProc == NULL) ||
						(gREXSetOutputSampleRateProc == NULL) ||
						(gREXRenderSliceProc == NULL) ||
						(gREXStartPreviewProc == NULL) ||
						(gREXStopPreviewProc == NULL) ||
						(gREXRenderPreviewBatchProc == NULL) ||
						(gREXSetPreviewTempoProc == NULL)) 
					{
						ClearProcPointers();
						UnloadREXDLL();
						return kREXError_UnableToLoadDLL;
					}
					else {
						if (gDLLOpenProc() != 0) {
							return kREXError_NoError;
						}
						else {
							ClearProcPointers();
							UnloadREXDLL();
							return kREXError_UnableToLoadDLL;
						}
					}
				}
			}
			else {
				return result;
			}
		}
	}
}




void REXUninitializeDLL(void) {
	/* REXLoadDLL() should only be called once, before REXUnloadDLL() is called! */
	/* REXUnloadDLL() should not be called unless a call to REXLoadDLL() has succeeded! */
	REX_ASSERT(gREXLoadCount == 1);
	if (gREXLoadCount > 0) {
		gREXLoadCount--;
	}
	if (gREXLoadCount == 0) {
		if(gDLLCloseProc != 0){
			gDLLCloseProc();
		}
		ClearProcPointers();
		UnloadREXDLL();
	}
}







#ifdef __cplusplus
}		/* End of namespace REX */
#endif /* def __cplusplus */
