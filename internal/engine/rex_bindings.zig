const std = @import("std");

pub const REX_int32_t = c_int;

pub const REXError = c_int;

pub const kREXError_NoError = 1;
pub const kREXError_OperationAbortedByUser = 2;
pub const kREXError_NoCreatorInfoAvailable = 3;
pub const kREXError_NotEnoughMemoryForDLL = 100;
pub const kREXError_UnableToLoadDLL = 101;
pub const kREXError_DLLTooOld = 102;
pub const kREXError_DLLNotFound = 103;
pub const kREXError_APITooOld = 104;
pub const kREXError_OutOfMemory = 105;
pub const kREXError_FileCorrupt = 106;
pub const kREXError_REX2FileTooNew = 107;
pub const kREXError_FileHasZeroLoopLength = 108;
pub const kREXError_OSVersionNotSupported = 109;
pub const kREXImplError_DLLNotInitialized = 200;
pub const kREXImplError_DLLAlreadyInitialized = 201;
pub const kREXImplError_InvalidHandle = 202;
pub const kREXImplError_InvalidSize = 203;
pub const kREXImplError_InvalidArgument = 204;
pub const kREXImplError_InvalidSlice = 205;
pub const kREXImplError_InvalidSampleRate = 206;
pub const kREXImplError_BufferTooSmall = 207;
pub const kREXImplError_IsBeingPreviewed = 208;
pub const kREXImplError_NotBeingPreviewed = 209;
pub const kREXImplError_InvalidTempo = 210;
pub const kREXError_Undefined = 666;

pub const REXHandle = ?*anyopaque;

pub const REXInfo = extern struct {
    fChannels: REX_int32_t,
    fSampleRate: REX_int32_t,
    fSliceCount: REX_int32_t,
    fTempo: REX_int32_t,
    fOriginalTempo: REX_int32_t,
    fPPQLength: REX_int32_t,
    fTimeSignNom: REX_int32_t,
    fTimeSignDenom: REX_int32_t,
    fBitDepth: REX_int32_t,
};

pub const REXSliceInfo = extern struct {
    fPPQPos: REX_int32_t,
    fSampleLength: REX_int32_t,
};

pub const REXCallbackResult = c_int;
pub const kREXCallback_Abort = 1;
pub const kREXCallback_Continue = 2;

pub const REXCreateCallback = *const fn (REX_int32_t, ?*anyopaque) callconv(.c) REXCallbackResult;

pub extern fn REXInitializeDLL() REXError;

// REXInitializeDLL_DirPath signature differs by OS:
//   Windows: const wchar_t* (u16)
//   Mac:     const char* UTF8 (u8)
pub extern fn REXInitializeDLL_DirPath(iDirPath: [*:0]const u16) REXError;
pub extern fn REXUninitializeDLL() void;
pub extern fn REXCreate(handle: *REXHandle, buffer: [*]const u8, size: REX_int32_t, callbackFunc: ?REXCreateCallback, userData: ?*anyopaque) REXError;
pub extern fn REXDelete(handle: *REXHandle) void;
pub extern fn REXGetInfo(handle: REXHandle, infoSize: REX_int32_t, info: *REXInfo) REXError;
pub extern fn REXGetSliceInfo(handle: REXHandle, sliceIndex: REX_int32_t, sliceInfoSize: REX_int32_t, sliceInfo: *REXSliceInfo) REXError;
pub extern fn REXSetOutputSampleRate(handle: REXHandle, outputSampleRate: REX_int32_t) REXError;
pub extern fn REXRenderSlice(handle: REXHandle, sliceIndex: REX_int32_t, bufferFrameLength: REX_int32_t, outputBuffers: [*][*c]f32) REXError;
pub extern fn REXStartPreview(handle: REXHandle) REXError;
pub extern fn REXStopPreview(handle: REXHandle) REXError;
pub extern fn REXRenderPreviewBatch(handle: REXHandle, framesToRender: REX_int32_t, outputBuffers: [*][*c]f32) REXError;
pub extern fn REXSetPreviewTempo(handle: REXHandle, tempo: REX_int32_t) REXError;
