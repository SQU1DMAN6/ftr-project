import curses
import random
import time
import socket
from scapy.all import ARP, Ether, srp
import threading
import os
import sys

if os.geteuid() != 0:
    print("This must be run as root.")
    sys.exit(1)

class MatrixIP:
    def __init__(self, y, x, ip, interval=3):
        self.y = y
        self.x = x
        self.ip = ip
        self.frames = 0
        self.interval = interval

    def tick(self):
        self.frames += 1
        if self.frames >= self.interval:
            self.y += 1
            self.frames = 0

class IpScanner:
    def __init__(self, network="192.168.1.0/24"):
        self.network = network
        self.ips = []
        self.lock = threading.Lock()
        self.running = True
        self.thread = threading.Thread(target=self.scan_loop, daemon=True)
        self.thread.start()

    def scan_loop(self):
        while self.running:
            arp = ARP(pdst=self.network)
            ether = Ether(dst="ff:ff:ff:ff:ff:ff")
            packet = ether / arp
            result = srp(packet, timeout=2, verbose=0)[0]
            ips = []
            for sent, received in result:
                ip = received.psrc
                try:
                    hostname = socket.gethostbyaddr(ip)[0]
                except:
                    hostname = "unknown"
                ips.append(f"{ip} - {hostname}")
            with self.lock:
                self.ips = ips
            time.sleep(10)

    def get_ips(self):
        with self.lock:
            return list(self.ips)

    def stop(self):
        self.running = False
        self.thread.join()

def init_matrix(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    curses.init_pair(1, curses.COLOR_GREEN, -1)   # body
    curses.init_pair(2, curses.COLOR_CYAN, -1)    # eye
    curses.init_pair(3, curses.COLOR_WHITE, -1)   # head white
    curses.init_pair(4, curses.COLOR_RED, -1)     # head red/IPs

    height, width = stdscr.getmaxyx()
    chars = list("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ@#$%&*+=<>|!?")

    streams = []
    for x in range(width):
        tail_length = random.randint(5, 20)
        head_y = random.randint(-height, 0)
        stream_chars = [random.choice(chars) for _ in range(tail_length)]
        streams.append({'x': x, 'y': head_y, 'tail_length': tail_length, 'chars': stream_chars})

    eye_open = [
        r"          ___.-------.___",
        r"      _.-' ___.--:--.___ `-._",
        r"   .-' _.-'  /  '+'  \  `-._ `-.",
        r" .' .-'      |-|=O=|-|      `-. `.",
        r"(_ <:__      \  '+'  /      __:> _)",
        r" `--._``-...__`._|_.'__...-'_.--'  ",
        r"      `- --._________.----'        "
    ]
    eye_closed = [
        r"          ___.-------.___",
        r"      _.-' ___.--:--.___ `-._",
        r"   .-' _.-'             `-._ `-.",
        r" .' .-'        -----        `-. `.",
        r"(_ <:__                     __:> _)",
        r" `--._``-...___________...-'_.--'",
        r"      `- --._________.----'"
    ]

    last_blink = time.time()
    blink_duration = 0.3
    blink_interval = 5
    eye_state = 'open'

    scanner = IpScanner()
    matrix_ips = []
    last_matrix_ip_spawn = time.time()

    try:
        while True:
            stdscr.erase()
            height, width = stdscr.getmaxyx()

            for stream in streams:
                stream['y'] = (stream['y'] + 1) % (height + stream['tail_length'])
                new_char = random.choice(chars)
                stream['chars'].insert(0, new_char)
                if len(stream['chars']) > stream['tail_length']:
                    stream['chars'].pop()

                for i, char in enumerate(stream['chars']):
                    y = stream['y'] - i
                    if 0 <= y < height:
                        if i == 0:
                            color = curses.color_pair(random.choice([3, 4]))
                        else:
                            color = curses.color_pair(1)
                        try:
                            stdscr.addstr(y, stream['x'], char, color)
                        except:
                            pass

            now = time.time()
            if eye_state == 'open' and (now - last_blink) > blink_interval:
                eye_state = 'closing'
                last_blink = now
            elif eye_state == 'closing' and (now - last_blink) > blink_duration:
                eye_state = 'open'
                last_blink = now

            eye = eye_open if eye_state == 'open' else eye_closed
            draw_eye(stdscr, eye, get_eye_shape_coords(height, width, eye))

            if now - last_matrix_ip_spawn > 4:
                ips = scanner.get_ips()
                if ips:
                    ip_text = random.choice(ips)
                    matrix_ips.append(MatrixIP(y=0, x=random.randint(0, width - len(ip_text)), ip=ip_text))
                last_matrix_ip_spawn = now

            for ip_obj in matrix_ips[:]:
                ip_obj.tick()
                if ip_obj.y < height:
                    try:
                        stdscr.addstr(ip_obj.y, ip_obj.x, ip_obj.ip[:width - ip_obj.x], curses.color_pair(4))
                    except:
                        pass
                else:
                    matrix_ips.remove(ip_obj)

            stdscr.refresh()
            time.sleep(0.05)

    except KeyboardInterrupt:
        scanner.stop()

def get_eye_shape_coords(height, width, eye):
    top = height // 2 - len(eye) // 2
    left = width // 2 - len(eye[0]) // 2
    coords = set()
    for dy, line in enumerate(eye):
        for dx, ch in enumerate(line):
            if ch != ' ':
                y, x = top + dy, left + dx
                if 0 <= y < height and 0 <= x < width:
                    coords.add((y, x))
    return coords

def draw_eye(stdscr, eye_lines, coords):
    if not coords:
        return
    top = min(y for y, _ in coords)
    left = min(x for _, x in coords)
    for dy, line in enumerate(eye_lines):
        try:
            stdscr.addstr(top + dy, left, line, curses.color_pair(2))
        except:
            pass

if __name__ == "__main__":
    curses.wrapper(init_matrix)
