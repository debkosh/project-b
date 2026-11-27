#!/bin/bash
# Makes the small streams core/streams' tests save (docs/video-streams.md): ffmpeg's test
# picture and a sine tone, a few seconds, cut into HLS in each form the app reads. Run it
# from this folder with ffmpeg on PATH, or FFMPEG naming it; what it writes is committed, so
# the tests need no ffmpeg. expect.json is what ffmpeg itself makes of each stream as one
# MP4, frame by frame (mp4check/expect), which the tests hold the app's MP4 to.
#
#   ts/       MPEG-TS pieces, picture and sound together, with B-frames
#   fmp4/     MP4 fragments with an EXT-X-MAP header piece
#   aes/      the ts pieces locked with AES-128: one key with its IV in the playlist, then
#             another with none, so the IV is the piece's sequence number
#   split/    a master playlist whose picture and sound are separate playlists of MP4
#             fragments, and a second, smaller quality
#   small/    the smaller quality's pieces, MPEG-TS
set -euo pipefail
cd "$(dirname "$0")"
FFMPEG=${FFMPEG:-ffmpeg}
ff() { "$FFMPEG" -hide_banner -loglevel error -nostdin -y "$@"; }
picture() { echo -f lavfi -i "testsrc=size=$1:rate=25" ; }
tone() { echo -f lavfi -i "sine=frequency=440:sample_rate=44100"; }
video="-c:v libx264 -preset veryslow -pix_fmt yuv420p -g 25 -keyint_min 25 -sc_threshold 0 -bf 2 -b:v 30k -x264-params bframes=2:b-adapt=0:nal-hrd=none"
audio="-c:a aac -b:a 16k -ac 1"
exact="-fflags +bitexact -flags:v +bitexact -flags:a +bitexact -map_metadata -1"

for d in ts fmp4 aes split small; do rm -rf "${d:?}"; mkdir "$d"; done

# ts: 4 seconds in 1 second pieces.
ff $(picture 160x90) $(tone) -t 4 $video $audio $exact \
  -f hls -hls_time 1 -hls_playlist_type vod -hls_segment_filename 'ts/piece%d.ts' ts/index.m3u8

# fmp4: the same, as fragments.
ff $(picture 160x90) $(tone) -t 4 $video $audio $exact \
  -f hls -hls_time 1 -hls_playlist_type vod -hls_segment_type fmp4 -hls_fmp4_init_filename init.mp4 \
  -hls_segment_filename 'fmp4/piece%d.m4s' fmp4/index.m3u8

# aes: the ts pieces again, each locked whole with AES-128-CBC, as HLS has it. Fixed keys,
# so the files come out the same each time.
printf '0123456789abcdef' > aes/one.key
printf 'fedcba9876543210' > aes/two.key
lock() { # piece, key file, iv in hex
  openssl aes-128-cbc -e -in "ts/piece$1.ts" -out "aes/piece$1.ts" -K "$(xxd -p "$2")" -iv "$3"
}
lock 0 aes/one.key 000102030405060708090a0b0c0d0e0f
lock 1 aes/one.key 000102030405060708090a0b0c0d0e0f
lock 2 aes/two.key 00000000000000000000000000000009   # sequence number 9: 7 + 2
lock 3 aes/two.key 0000000000000000000000000000000a
cat > aes/index.m3u8 <<'LIST'
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:1
#EXT-X-MEDIA-SEQUENCE:7
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-KEY:METHOD=AES-128,URI="one.key",IV=0x000102030405060708090A0B0C0D0E0F
#EXTINF:1.000000,
piece0.ts
#EXTINF:1.000000,
piece1.ts
#EXT-X-KEY:METHOD=AES-128,URI="two.key"
#EXTINF:1.000000,
piece2.ts
#EXTINF:1.000000,
piece3.ts
#EXT-X-ENDLIST
LIST

# split: picture and sound in playlists of their own, MP4 fragments, 3 seconds.
ff $(picture 160x90) -t 3 $video -an $exact \
  -f hls -hls_time 1 -hls_playlist_type vod -hls_segment_type fmp4 -hls_fmp4_init_filename video-init.mp4 \
  -hls_segment_filename 'split/video%d.m4s' split/video.m3u8
ff $(tone) -t 3 -vn $audio $exact \
  -f hls -hls_time 1 -hls_playlist_type vod -hls_segment_type fmp4 -hls_fmp4_init_filename audio-init.mp4 \
  -hls_segment_filename 'split/audio%d.m4s' split/audio.m3u8
# small: the lesser quality, 2 seconds of a smaller picture.
ff $(picture 96x54) $(tone) -t 2 $video $audio $exact \
  -f hls -hls_time 1 -hls_playlist_type vod -hls_segment_filename 'small/piece%d.ts' small/index.m3u8
# The master lists the small one first and gives it the greater bandwidth: the best is the
# larger picture.
cat > split/master.m3u8 <<'LIST'
#EXTM3U
#EXT-X-VERSION:7
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="sound",NAME="Tone",DEFAULT=YES,AUTOSELECT=YES,URI="audio.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=900000,RESOLUTION=96x54,CODECS="avc1.42c00a,mp4a.40.2"
../small/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=60000,RESOLUTION=160x90,CODECS="avc1.64000a,mp4a.40.2",AUDIO="sound"
video.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=50000,RESOLUTION=160x90,CODECS="avc1.64000a,mp4a.40.2",AUDIO="sound"
video.m3u8?lesser
LIST

# What ffmpeg makes of each as one MP4, for expect.json.
ref=$(mktemp -d)
trap 'rm -rf "${ref:?}"' EXIT
ff -i ts/index.m3u8 -c copy "$ref/ts.mp4"
ff -i fmp4/index.m3u8 -c copy "$ref/fmp4.mp4"
ff -i split/video.m3u8 -i split/audio.m3u8 -c copy "$ref/split.mp4"
ff -i small/index.m3u8 -c copy "$ref/small.mp4"
(cd ../.. && go run ./streams/mp4check/expect "$ref/ts.mp4" "$ref/fmp4.mp4" "$ref/split.mp4" "$ref/small.mp4") > expect.json
du -sh . | cut -f1
