import tkinter as tk
from tkinter import scrolledtext, messagebox
import numpy as np
import pyaudio
import threading
import os
import time

os.environ["PYTHONWARNINGS"] = "ignore"
os.environ["ALSA_CARD"] = "default"

# Silence ALSA errors
import ctypes
try:
    asound = ctypes.cdll.LoadLibrary("libasound.so")
    asound.snd_lib_error_set_handler(None)
except:
    pass

# Parameters
BAUD_RATE = 8
F_MARK = 2600  # 1s
F_SPACE = 2400  # 0s
SAMPLE_RATE = 44100
CHUNK = 1024
RECORD_SECONDS = 10

PREAMBLE = '10101010' * 2  # sync

def string_to_bits(s: str) -> str:
    return ''.join(format(ord(c), '08b') for c in s)

def afsk_modulate_and_play(bits: str, on_bit=None):
    p = pyaudio.PyAudio()
    stream = p.open(format=pyaudio.paFloat32,
                    channels=1,
                    rate=SAMPLE_RATE,
                    output=True)

    for bit in bits:
        freq = F_MARK if bit == '1' else F_SPACE
        t = np.linspace(0, 1 / BAUD_RATE, int(SAMPLE_RATE / BAUD_RATE), endpoint=False)
        wave = 0.5 * np.sin(2 * np.pi * freq * t).astype(np.float32)
        stream.write(wave.tobytes())
        if on_bit:
            on_bit(bit)

    stream.stop_stream()
    stream.close()
    p.terminate()

def record_audio(duration=RECORD_SECONDS):
    p = pyaudio.PyAudio()
    stream = p.open(format=pyaudio.paInt16,
                    channels=1,
                    rate=SAMPLE_RATE,
                    input=True,
                    frames_per_buffer=CHUNK)
    frames = []
    for _ in range(0, int(SAMPLE_RATE / CHUNK * duration)):
        data = stream.read(CHUNK)
        frames.append(data)
    stream.stop_stream()
    stream.close()
    p.terminate()
    return b''.join(frames)

def goertzel(samples, freq, sample_rate):
    s_prev = 0
    s_prev2 = 0
    normalized_freq = freq / sample_rate
    coeff = 2 * np.cos(2 * np.pi * normalized_freq)
    for sample in samples:
        s = sample + coeff * s_prev - s_prev2
        s_prev2 = s_prev
        s_prev = s
    power = s_prev2**2 + s_prev**2 - coeff * s_prev * s_prev2
    return power

def afsk_demodulate(audio_data: bytes) -> str:
    audio = np.frombuffer(audio_data, dtype=np.int16).astype(np.float32)
    audio /= 32768.0
    samples_per_bit = int(SAMPLE_RATE / BAUD_RATE)
    bits = []

    for i in range(0, len(audio), samples_per_bit):
        chunk = audio[i:i + samples_per_bit]
        if len(chunk) < samples_per_bit:
            break
        power_mark = goertzel(chunk, F_MARK, SAMPLE_RATE)
        power_space = goertzel(chunk, F_SPACE, SAMPLE_RATE)
        bit = '1' if power_mark > power_space else '0'
        bits.append(bit)

    all_bits = ''.join(bits)
    preamble_idx = all_bits.find(PREAMBLE)
    if preamble_idx == -1:
        return "No sync found (preamble missing)."

    bits = bits[preamble_idx + len(PREAMBLE):]
    return ''.join(bits)

class RWChanApp:
    def __init__(self, root):
        self.root = root
        root.title("RW-Chan Bitstream Messenger")

        tk.Label(root, text="Message to send:").pack()
        self.send_entry = tk.Entry(root, width=50)
        self.send_entry.pack()

        self.send_button = tk.Button(root, text="Send", command=self.send_message)
        self.send_button.pack()

        tk.Label(root, text="Transmission Log:").pack()
        self.receive_text = scrolledtext.ScrolledText(root, height=15, width=60)
        self.receive_text.pack()

        self.receive_button = tk.Button(root, text="Receive", command=self.receive_message)
        self.receive_button.pack()

        self.byte_buffer = ""

    def send_message(self):
        msg = self.send_entry.get()
        if not msg:
            messagebox.showwarning("Empty message", "Please enter a message to send!")
            return

        self.send_button.config(state='disabled')
        self.receive_text.insert(tk.END, f"[Sending]: {msg}\n[Bitstream]: ")
        self.receive_text.see(tk.END)
        self.byte_buffer = ""

        bits = PREAMBLE + string_to_bits(msg)

        def display_bit(bit):
            self.receive_text.insert(tk.END, bit)
            self.byte_buffer += bit
            if len(self.byte_buffer) == 8:
                self.receive_text.insert(tk.END, ' ')
                self.byte_buffer = ''
            self.receive_text.see(tk.END)
            self.receive_text.update_idletasks()

        def send_thread():
            afsk_modulate_and_play(bits, on_bit=display_bit)
            if self.byte_buffer:
                self.receive_text.insert(tk.END, ' ')
                self.byte_buffer = ''
            self.receive_text.insert(tk.END, "\n[Done]\n")
            self.send_button.config(state='normal')

        threading.Thread(target=send_thread, daemon=True).start()

    def receive_message(self):
        self.receive_button.config(state='disabled')
        self.receive_text.insert(tk.END, "[Recording...]\n")
        self.receive_text.see(tk.END)

        def record_and_decode():
            audio_data = record_audio()
            decoded_bits = afsk_demodulate(audio_data)
            grouped = ' '.join(decoded_bits[i:i+8] for i in range(0, len(decoded_bits), 8))
            self.receive_text.insert(tk.END, f"[Received bits]: {grouped}\n")
            self.receive_text.see(tk.END)
            self.receive_button.config(state='normal')

        threading.Thread(target=record_and_decode, daemon=True).start()

if __name__ == "__main__":
    root = tk.Tk()
    app = RWChanApp(root)
    root.mainloop()
