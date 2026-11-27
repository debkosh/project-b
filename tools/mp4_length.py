"""What a saved video is, read from the MP4 itself, for the real-app checks of a streamed
video saved as one MP4 (docs/video-streams.md): its length in seconds as its header says,
the kinds of its tracks, and whether its index comes before its data, as a regular MP4's
does. No player and no ffmpeg needed."""
import struct


def boxes(data, start=0, end=None):
    """Each box in data[start:end], as (type, body start, body end)."""
    end = len(data) if end is None else end
    at = start
    while at + 8 <= end:
        size, kind = struct.unpack(">I4s", data[at:at + 8])
        body = at + 8
        if size == 1:
            size = struct.unpack(">Q", data[at + 8:at + 16])[0]
            body = at + 16
        elif size == 0:
            size = end - at
        if size < 8 or at + size > end:
            return
        yield kind.decode("latin-1"), body, at + size
        at += size


def read(path):
    """{"seconds", "tracks", "index_first"} of the MP4 at path; seconds is 0 and tracks
    empty for a file that isn't one."""
    with open(path, "rb") as f:
        data = f.read()
    out = {"seconds": 0.0, "tracks": [], "index_first": False}
    seen_data = False
    for kind, start, end in boxes(data):
        if kind == "mdat":
            seen_data = True
        if kind != "moov":
            continue
        out["index_first"] = not seen_data
        for inner, a, b in boxes(data, start, end):
            if inner == "mvhd":
                if data[a] == 1:
                    scale, length = struct.unpack(">IQ", data[a + 20:a + 32])
                else:
                    scale, length = struct.unpack(">II", data[a + 12:a + 20])
                out["seconds"] = length / scale if scale else 0.0
            if inner == "trak":
                for mdia, c, d in boxes(data, a, b):
                    if mdia == "mdia":
                        for hdlr, e, _ in boxes(data, c, d):
                            if hdlr == "hdlr":
                                out["tracks"].append(data[e + 8:e + 12].decode("latin-1"))
    return out
