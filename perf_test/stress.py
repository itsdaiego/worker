import sys
import time
import threading
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed

BASE = "http://localhost:8080"

JOB_TYPES = ["send_email", "resize_image", "generate_report"]
TARGET_JOBS = 1_000
CREATE_CONCURRENCY = 50

created = 0
create_errors = 0
lock = threading.Lock()

PASS = "\033[92mPASS\033[0m"
FAIL = "\033[91mFAIL\033[0m"
results: list[tuple[str, bool, str]] = []


def check(label: str, passed: bool, detail: str = "") -> bool:
    results.append((label, passed, detail))
    status = PASS if passed else FAIL
    print(f"  [{status}] {label}{' — ' + detail if detail else ''}")
    return passed


def create_single_job(i: int):
    global created, create_errors
    job_type = JOB_TYPES[i % 3]
    try:
        r = requests.post(
            f"{BASE}/jobs",
            json={"type": job_type, "payload": f"stress_{i}"},
            timeout=10,
        )
        if r.status_code == 201:
            with lock:
                created += 1
        else:
            with lock:
                create_errors += 1
    except Exception:
        with lock:
            create_errors += 1


def get_job_stats() -> dict:
    try:
        jobs = requests.get(f"{BASE}/jobs", timeout=30).json()
        stats = {"pending": 0, "done": 0, "failed": 0, "total": len(jobs)}
        for j in jobs:
            s = j.get("status", "unknown")
            if s in stats:
                stats[s] += 1
        return stats
    except Exception as e:
        return {"error": str(e)}


def monitor(stop_event: threading.Event):
    while not stop_event.is_set():
        stats = get_job_stats()
        if "error" in stats:
            print(f"\r  \033[90m[monitor] error: {stats['error']}\033[0m", end="", flush=True)
        else:
            bar_width = 40
            total = stats["total"] or 1
            done_bars = int((stats["done"] / total) * bar_width)
            fail_bars = int((stats["failed"] / total) * bar_width)
            pend_bars = bar_width - done_bars - fail_bars

            bar = (
                f"\033[92m{'█' * done_bars}\033[0m"
                f"\033[91m{'█' * fail_bars}\033[0m"
                f"\033[90m{'░' * pend_bars}\033[0m"
            )

            print(
                f"\r  [{bar}] "
                f"total={stats['total']:,} "
                f"\033[92mdone={stats['done']:,}\033[0m "
                f"\033[91mfail={stats['failed']:,}\033[0m "
                f"\033[93mpend={stats['pending']:,}\033[0m   ",
                end="",
                flush=True,
            )
        time.sleep(0.5)


def main():
    try:
        requests.get(BASE, timeout=2)
    except Exception:
        print(f"Server not reachable at {BASE}. Start it with: go run .")
        sys.exit(1)

    print(f"\n{'='*60}")
    print(f"  STRESS TEST — {TARGET_JOBS:,} jobs")
    print(f"{'='*60}")

    print(f"\n=== Phase 1: Bulk Creation ({TARGET_JOBS:,} jobs, {CREATE_CONCURRENCY} threads) ===")
    t0 = time.perf_counter()

    with ThreadPoolExecutor(max_workers=CREATE_CONCURRENCY) as pool:
        futures = [pool.submit(create_single_job, i) for i in range(TARGET_JOBS)]
        for f in as_completed(futures):
            f.result()

    t_create = time.perf_counter() - t0
    create_rate = created / t_create if t_create > 0 else 0

    print(f"  Created {created:,} jobs in {t_create:.2f}s ({create_rate:.0f}/sec)")
    check("all jobs created successfully", create_errors == 0, f"{create_errors} errors")
    check("creation throughput > 100/sec", create_rate > 100, f"{create_rate:.0f}/sec")
    check("creation throughput > 500/sec", create_rate > 500, f"{create_rate:.0f}/sec")

    print(f"\n=== Phase 2: Batch Processing ===")
    stop_monitor = threading.Event()
    mon = threading.Thread(target=monitor, args=(stop_monitor,), daemon=True)
    mon.start()

    t0 = time.perf_counter()
    try:
        r = requests.post(f"{BASE}/jobs/batch", timeout=300)
        t_batch = time.perf_counter() - t0
        body = r.json()
    except requests.exceptions.Timeout:
        t_batch = 300
        body = {}
        r = None

    stop_monitor.set()
    mon.join(timeout=2)
    print()

    batch_total = body.get("total", 0)
    batch_succeeded = body.get("succeeded", 0)
    batch_failed = body.get("failed", 0)
    batch_rate = batch_total / t_batch if t_batch > 0 else 0

    print(f"  Batch: {batch_total:,} jobs in {t_batch:.2f}s ({batch_rate:.0f}/sec)")
    print(f"  Succeeded: {batch_succeeded:,} | Failed: {batch_failed:,}")

    check("batch returns 200", r is not None and r.status_code == 200)
    check("batch processed all pending jobs", batch_total == created, f"expected {created}, got {batch_total}")
    check("zero failures", batch_failed == 0, f"{batch_failed} failed")
    check("batch completes in < 120s", t_batch < 120, f"{t_batch:.2f}s")
    check("batch completes in < 30s", t_batch < 30, f"{t_batch:.2f}s")
    check("batch completes in < 10s", t_batch < 10, f"{t_batch:.2f}s")
    check("batch throughput > 50/sec", batch_rate > 50, f"{batch_rate:.0f}/sec")
    check("batch throughput > 200/sec", batch_rate > 200, f"{batch_rate:.0f}/sec")
    check("batch throughput > 1000/sec", batch_rate > 1000, f"{batch_rate:.0f}/sec")

    print(f"\n=== Phase 3: Post-batch Verification ===")
    stats = get_job_stats()
    check("no pending jobs remain", stats.get("pending", -1) == 0, f"{stats.get('pending', '?')} pending")
    check("all jobs marked done", stats.get("done", 0) == created, f"{stats.get('done', 0)}/{created}")

    total = len(results)
    passed = sum(1 for _, ok, _ in results if ok)
    failed = total - passed

    print(f"\n{'='*60}")
    print(f"  Results: {passed}/{total} passed", end="")
    if failed:
        print(f"  ({failed} failed)")
    else:
        print("  — all good")
    print(f"{'='*60}\n")

    sys.exit(0 if failed == 0 else 1)


if __name__ == "__main__":
    main()
