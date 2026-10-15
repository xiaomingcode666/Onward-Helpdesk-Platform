export async function convertAudioToPCM16Wav(file: File): Promise<File> {
  const AudioContextConstructor =
    window.AudioContext ||
    (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
  if (!AudioContextConstructor) {
    throw new Error("This browser cannot decode audio for transcription.")
  }

  const context = new AudioContextConstructor()
  try {
    const decoded = await context.decodeAudioData(await file.arrayBuffer())
    const sampleRate = 16_000
    const frameCount = Math.max(1, Math.ceil(decoded.duration * sampleRate))
    const pcm = new ArrayBuffer(frameCount * 2)
    const view = new DataView(pcm)
    for (let frame = 0; frame < frameCount; frame += 1) {
      const sourceIndex = Math.min(decoded.length - 1, Math.floor((frame * decoded.sampleRate) / sampleRate))
      let sample = 0
      for (let channel = 0; channel < decoded.numberOfChannels; channel += 1) {
        sample += decoded.getChannelData(channel)[sourceIndex] || 0
      }
      sample /= decoded.numberOfChannels
      const clipped = Math.max(-1, Math.min(1, sample))
      view.setInt16(frame * 2, clipped < 0 ? clipped * 0x8000 : clipped * 0x7fff, true)
    }

    const wav = new ArrayBuffer(44 + pcm.byteLength)
    const wavView = new DataView(wav)
    writeASCII(wavView, 0, "RIFF")
    wavView.setUint32(4, 36 + pcm.byteLength, true)
    writeASCII(wavView, 8, "WAVE")
    writeASCII(wavView, 12, "fmt ")
    wavView.setUint32(16, 16, true)
    wavView.setUint16(20, 1, true)
    wavView.setUint16(22, 1, true)
    wavView.setUint32(24, sampleRate, true)
    wavView.setUint32(28, sampleRate * 2, true)
    wavView.setUint16(32, 2, true)
    wavView.setUint16(34, 16, true)
    writeASCII(wavView, 36, "data")
    wavView.setUint32(40, pcm.byteLength, true)
    new Uint8Array(wav, 44).set(new Uint8Array(pcm))
    return new File([wav], `${file.name.replace(/\.[^.]+$/, "") || "voice"}.wav`, {
      type: "audio/wav",
      lastModified: Date.now(),
    })
  } finally {
    await context.close()
  }
}

function writeASCII(view: DataView, offset: number, value: string) {
  for (let index = 0; index < value.length; index += 1) {
    view.setUint8(offset + index, value.charCodeAt(index))
  }
}
