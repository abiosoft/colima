const std = @import("std");
const File = std.fs.File;

const Stat = struct { atime: i128, mtime: i128 };

pub fn main() !void {
    if (std.os.argv.len < 2) {
        std.debug.print("len(args) < 1: {any}\n", .{std.os.argv});
        std.os.exit(1);
    }

    const f_path = std.mem.span(std.os.argv[1]);

    const file = try openFile(f_path, .{ .mode = .read_write });
    defer file.close();

    const stat = try file.stat();
    const st = Stat{ .atime = stat.atime, .mtime = stat.mtime };

    try file.updateTimes(st.atime, st.mtime);

    println("updated time for {s} to atime: {}, ctime: {}", .{ f_path, st.atime, st.mtime });
}

fn openFile(file_path: []const u8, flags: File.OpenFlags) File.OpenError!File {
    return if (std.fs.path.isAbsolute(file_path))
        std.fs.openFileAbsolute(file_path, flags)
    else
        std.fs.cwd().openFile(file_path, flags);
}

fn println(comptime fmt: []const u8, args: anytype) void {
    std.io.getStdOut().writer().print(fmt ++ "\n", args) catch return;
}
