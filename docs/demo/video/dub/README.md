# Indonesian dub pipeline

Produces `../SELA-Product-Demo-3min-1080p-ID-dub-EN-subs.mp4` from the English-subtitled
master and `../DUB_SCRIPT_ID.md`. Needs `ffmpeg`, `python3` and `pip install edge-tts`.
Run every step from one empty working directory that has a `work/` subfolder.

```sh
export SRC_VIDEO=/path/to/SELA-Product-Demo-3min-1080p-EN-subs.mp4
D=/path/to/sela/docs/demo/video
mkdir -p lines trim tight best work

# 1. TTS, one file per script row (writes lines/NN.mp3 + lines/plan.json)
python3 $D/dub/tts.py $D/DUB_SCRIPT_ID.md lines id-ID-GadisNeural   # or id-ID-ArdiNeural

# 2. Strip edge silence (trim/), and also cap inner pauses at 0.22 s (tight/)
for f in lines/*.mp3; do n=$(basename $f .mp3)
  ffmpeg -v error -y -i $f -af "silenceremove=start_periods=1:start_threshold=-45dB,areverse,silenceremove=start_periods=1:start_threshold=-45dB,areverse" -ar 48000 -ac 1 trim/$n.wav
  ffmpeg -v error -y -i trim/$n.wav -af "silenceremove=stop_periods=-1:stop_duration=0.22:stop_threshold=-42dB" tight/$n.wav
done

# 3. Copy into best/: trim/NN.wav when it fits its slot with 0.6 s to spare, else tight/NN.wav

# 4. Schedule: start up to 0.45 s early / 0.5 s late, speed-up only when needed
python3 $D/dub/sched.py best 9 182.6     # writes best/sched.json; max speed used was 1.185x

# 5. Voice track (vo.wav) + music ducking envelope (-9 dB under voice)
python3 $D/dub/mix.py
```

Then mix (voice +2.5 dB over the ducked music), normalize to −16 LUFS / −1.5 dBTP
(the release used two-pass `loudnorm` with `linear=true`), and re-encode with the last frame
held 3 s (line 36 does not fit in the final 1.8 s):

```sh
printf '[0:a]apad=whole_dur=183,atrim=0:183,%s[m];[1:a]volume=2.5dB,aformat=channel_layouts=stereo,apad=whole_dur=183,atrim=0:183[v];[m][v]amix=inputs=2:normalize=0:duration=longest[mix]' "$(cat work/duck.txt)" > fc.txt
ffmpeg -i "$SRC_VIDEO" -i vo.wav -filter_complex_script fc.txt -map "[mix]" -ar 48000 premix.wav
ffmpeg -i premix.wav -af loudnorm=I=-16:TP=-1.5:LRA=11 -ar 48000 final_audio.wav
ffmpeg -i "$SRC_VIDEO" -i final_audio.wav \
  -filter_complex "[0:v]tpad=stop_mode=clone:stop_duration=3[v]" -map "[v]" -map 1:a \
  -c:v libx264 -preset medium -crf 18 -pix_fmt yuv420p -r 30 -c:a aac -b:a 192k -t 183 \
  -movflags +faststart SELA-Product-Demo-3min-1080p-ID-dub-EN-subs.mp4
```
