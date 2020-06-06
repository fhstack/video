#!/usr/bin/env bash
sudo dnctl pipe 1 config bw 20Mbit/s delay 50 plr 0.05 # 较差的网络
sudo dnctl pipe 2 config bw 10Mbit/s delay 50 plr 0.10 # 很差的网络