const std = @import("std");
const build_options = @import("build_options");

// Stub module for Linux builds — no REX SDK available.
// Exports same symbol names as extractor.zig but returns errors/nulls.

pub const ZigMetadata = extern struct {
    channels: i32,
    sample_rate: i32,
    tempo: f64,
    original_tempo: f64,
    time_sign_nom: i32,
    time_sign_denom: i32,
    bit_depth: i32,
    ppq_length: i32,
};

pub const ZigSlicePayload = extern struct {
    slice_index: i32,
    ppq_pos: i32,
    frame_length: i32,
    pcm_data: [*c]f32,
};

pub const ZigRawExtraction = extern struct {
    metadata: ZigMetadata,
    slice_count: i32,
    slices: [*c]ZigSlicePayload,
};

pub const ZigLoopSliceInfo = extern struct {
    ppq_pos: i32,
};

pub const ZigLoopRenderResult = extern struct {
    metadata: ZigMetadata,
    tempo: i32,
    frame_length: i32,
    slice_count: i32,
    slice_info: [*c]ZigLoopSliceInfo,
    pcm_data: [*c]f32,
};

pub const ZigPerSliceResult = extern struct {
    frame_length: i32,
    pcm_data: [*c]f32,
    ppq_pos: i32,
    sample_pos: i32,
};

pub const ZigSlicesRenderResult = extern struct {
    metadata: ZigMetadata,
    tempo: i32,
    total_frames: i32,
    slice_count: i32,
    slices: [*c]ZigPerSliceResult,
};

export fn Zig_Diagnostic() void {
    std.debug.print("REX SDK not available (Linux stub)\n", .{});
}

export fn Zig_InitEngine() i32 {
    return 0;
}

export fn Zig_CloseEngine() void {}

export fn Zig_ExtractRawData(_: [*c]const u8, _: i32, _: i32) ?*ZigRawExtraction {
    return null;
}

export fn Zig_FreeRawData(_: ?*ZigRawExtraction) void {}

export fn Zig_RenderLoopPreview(_: [*c]const u8, _: i32, _: i32, _: i32) ?*ZigLoopRenderResult {
    return null;
}

export fn Zig_FreeLoopRenderResult(_: ?*ZigLoopRenderResult) void {}

export fn Zig_RenderSlicesPreview(_: [*c]const u8, _: i32, _: i32, _: i32) ?*ZigSlicesRenderResult {
    return null;
}

export fn Zig_FreeSlicesRenderResult(_: ?*ZigSlicesRenderResult) void {}

pub fn main() !void {
    if (build_options.link_go) {
        const GoMainEntry = @extern(*const fn () callconv(.C) void, .{ .name = "GoMainEntry" });
        GoMainEntry();
    } else {
        // On Linux, the actual binary is built with `go build`. This Zig binary
        // exists only as a build artifact for project consistency across platforms.
    }
}
