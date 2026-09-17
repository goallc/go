#!/usr/bin/env bash
# Stream diagnostics as well as saving them: a stopped VM cannot upload artifacts.
set -u
while true; do
  echo "=== runner resources $(date -u +%FT%TZ) ==="
  grep -E '^(MemTotal|MemAvailable|SwapTotal|SwapFree):' /proc/meminfo
  grep '^oom_kill ' /proc/vmstat
  cat /proc/pressure/memory
  for path in /sys/fs/cgroup "/sys/fs/cgroup$(awk -F: '$1 == "0" {print $3}' /proc/self/cgroup)"; do
    for file in memory.current memory.peak memory.max memory.events; do
      if [[ -f "$path/$file" ]]; then
        echo "$path/$file"
        cat "$path/$file"
      fi
    done
  done
  df -h / /tmp
  ps -eo pid,ppid,rss,comm --sort=-rss | head -n 12
  # Compiler arguments identify the fixture without dumping runner credentials.
  ps -C compile -o pid=,rss=,args= || true
  sudo dmesg -T | grep -Ei 'out of memory|oom-kill|killed process|memory cgroup' | tail -n 15 || true
  sleep 10
done
