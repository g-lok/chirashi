const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{
        .default_target = .{
            .cpu_arch = .x86_64,
            .os_tag = .macos,
        },
    });
    const optimize = b.standardOptimizeOption(.{});
    const version = b.option([]const u8, "version", "Release version string (git describe)") orelse "dev";
    const go_archive = b.option([]const u8, "go-archive", "Path to pre-built Go c-archive (skips Go build step)") orelse "";

    const link_go = b.option(bool, "link-go", "Link Go c-archive into Zig binary") orelse (target.result.os.tag != .linux);
    const os_is_linux = target.result.os.tag == .linux;

    // --- Go static archive (macOS/Windows only) ---
    const go_build_step = if (link_go and go_archive.len == 0) blk: {
        const ldflags = std.mem.concat(b.allocator, u8, &.{ "-X ", "github", ".", "com/g-lok/rexconverter/cmd.version=", version }) catch @panic("OOM");

        const gb = b.addSystemCommand(&.{
            "go",                             "build",
            "-buildmode=c-archive",           "-tags",
            "netgo",                          "-ldflags",
            ldflags,                          "-o",
            "internal/rexengine/go_engine.a", "main.go",
        });
        gb.setEnvironmentVariable("CGO_ENABLED", "1");

        if (target.result.os.tag == .macos) {
            gb.setEnvironmentVariable("GOOS", "darwin");

            switch (target.result.cpu.arch) {
                .x86_64 => {
                    gb.setEnvironmentVariable("GOARCH", "amd64");
                    gb.setEnvironmentVariable("CC", "zig cc -target x86_64-macos");
                },
                .aarch64 => {
                    gb.setEnvironmentVariable("GOARCH", "arm64");
                    gb.setEnvironmentVariable("CC", "zig cc -target aarch64-macos");
                },
                else => {
                    gb.setEnvironmentVariable("GOARCH", "amd64");
                    gb.setEnvironmentVariable("CC", "zig cc -target x86_64-macos");
                },
            }
        } else if (target.result.os.tag == .windows) {
            gb.setEnvironmentVariable("GOOS", "windows");

            switch (target.result.cpu.arch) {
                .x86_64 => {
                    gb.setEnvironmentVariable("GOARCH", "amd64");
                    gb.setEnvironmentVariable("CC", "zig cc -target x86_64-windows-gnu");
                },
                else => {
                    gb.setEnvironmentVariable("GOARCH", "amd64");
                    gb.setEnvironmentVariable("CC", "zig cc -target x86_64-windows-gnu");
                },
            }
        } else {
            @panic("unsupported target OS (must be macOS or Windows) for --link-go");
        }

        break :blk gb;
    } else null;

    // --- Zig module ---
    // Linux: use stub (no REX SDK). macOS/Windows: use full extractor with REX.h.
    const zig_source = if (os_is_linux)
        b.path("internal/rexengine/extractor_stub.zig")
    else
        b.path("internal/rexengine/extractor.zig");

    const root_module = b.createModule(.{
        .root_source_file = zig_source,
        .target = target,
        .optimize = optimize,
        .link_libc = true,
    });

    // Build options: pass link_go flag to stub (not used by extractor.zig)
    const options = b.addOptions();
    options.addOption(bool, "link_go", link_go);
    root_module.addImport("build_options", options.createModule());

    if (!os_is_linux) {
        const rex_c = b.addTranslateC(.{
            .root_source_file = b.path("internal/rexengine/REX.h"),
            .target = target,
            .optimize = optimize,
            .link_libc = true,
        });
        if (target.result.os.tag == .windows) {
            rex_c.defineCMacro("REX_WINDOWS", "1");
            rex_c.defineCMacro("REX_MAC", "0");
        } else {
            rex_c.defineCMacro("REX_MAC", "1");
            rex_c.defineCMacro("REX_WINDOWS", "0");
        }
        root_module.addImport("rex_c", rex_c.createModule());
    }

    // --- Executable ---
    var exe = b.addExecutable(.{
        .name = "rexconverter",
        .root_module = root_module,
    });

    // Platform-specific SDK linking
    if (target.result.os.tag == .windows) {
        exe.root_module.addCSourceFile(.{ .file = b.path("internal/rexengine/rex/REX.c"), .flags = &.{ "-DREX_WINDOWS=1", "-DREX_MAC=0" } });
        exe.root_module.linkSystemLibrary("version", .{});
    } else if (target.result.os.tag == .macos) {
        exe.root_module.addFrameworkPath(b.path("internal/rexengine/libs/macos"));
        exe.root_module.linkFramework("REX Shared Library", .{});
        exe.headerpad_max_install_names = true;
        exe.root_module.addRPath(b.path("Frameworks"));
    } else if (os_is_linux) {
        // Linux: no REX SDK — uses extractor_stub.zig with link_go=false
    } else {
        @panic("unsupported target OS (must be macOS, Windows, or Linux)");
    }

    // Include path for REX.h (needed by REX.c on Windows)
    exe.root_module.addIncludePath(b.path("internal/rexengine"));

    // Link the Go static archive (macOS/Windows only)
    if (link_go) {
        const archive_path = if (go_archive.len > 0) b.path(go_archive) else b.path("internal/rexengine/go_engine.a");
        exe.root_module.addObjectFile(archive_path);

        if (go_build_step) |gs| {
            exe.step.dependOn(&gs.step);
        }

        // Windows: BSD-format archive fix
        if (target.result.os.tag == .windows) {
            const target_file = if (go_archive.len > 0) go_archive else "internal/rexengine/go_engine.a";
            const fix_cmd = b.fmt("ar x {s} 2>/dev/null; zig ar rcs {s} *.o 2>/dev/null; rm -f *.o 2>/dev/null", .{ target_file, target_file });
            const fix_archive = b.addSystemCommand(&.{ "sh", "-c", fix_cmd });
            if (go_build_step) |gs| {
                fix_archive.step.dependOn(&gs.step);
            }
            exe.step.dependOn(&fix_archive.step);
        }
    }

    b.installArtifact(exe);
}
