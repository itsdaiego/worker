import sys
import time
import threading
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed

BASE = "http://localhost:8080"

JOB_TYPES = ["send_email", "resize_image", "generate_report"]
TARGET_JOBS = 10_000
CREATE_CONCURRENCY = 100

PASS = "\033[92mPASS\033[0m"
FAIL = "\033[91mFAIL\033[0m"
results: list[tuple[str, bool, str]] = []


def check(label: str, passed: bool, detail: str = "") -> bool:
    results.append((label, passed, detail))
    status = PASS if passed else FAIL
    print(f"  [{status}] {label}{' — ' + detail if detail else ''}")
    return passed


def create_single_job(i: int):
    job_type = JOB_TYPES[i % 3]
    r = requests.post(f"{BASE}/jobs", json={"type": job_type, "payload": f"stress_{i}"}, timeout=10)
    return r.status_code == 201


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
        if "error" not in stats:
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

    # ── Phase 1: Create jobs ──
    print(f"\n=== Phase 1: Creating {TARGET_JOBS:,} jobs ===")
    t0 = time.perf_counter()
    created = 0

    with ThreadPoolExecutor(max_workers=CREATE_CONCURRENCY) as pool:
        futures = [pool.submit(create_single_job, i) for i in range(TARGET_JOBS)]
        for f in as_completed(futures):
            if f.result():
                created += 1

    t_create = time.perf_counter() - t0
    print(f"  Created {created:,} jobs in {t_create:.2f}s")

    # ── Phase 2: Batch Processing ──
    # Each job does time.Sleep(300ms). With N workers processing
    # 10,000 jobs, theoretical min = (10000/N) * 0.3s
    # 3 workers:   ~1000s
    # 100 workers:  ~30s
    # 1000 workers:  ~3s
    # 10000 workers: ~0.3s (all parallel)
    print(f"\n=== Phase 2: Batch Processing ({created:,} jobs, 300ms/job) ===")
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
    check("batch processed all jobs", batch_total == created, f"expected {created}, got {batch_total}")
    check("zero failures", batch_failed == 0, f"{batch_failed} failed")

    # time thresholds (tests parallelism quality)
    check("batch < 60s  (need ~167 workers)", t_batch < 60, f"{t_batch:.2f}s")
    check("batch < 30s  (need ~333 workers)", t_batch < 30, f"{t_batch:.2f}s")
    check("batch < 10s  (need ~1000 workers)", t_batch < 10, f"{t_batch:.2f}s")
    check("batch < 5s   (need ~2000 workers)", t_batch < 5, f"{t_batch:.2f}s")
    check("batch < 1s   (need ~10000 workers)", t_batch < 1, f"{t_batch:.2f}s")

    # throughput thresholds
    check("throughput > 500/sec", batch_rate > 500, f"{batch_rate:.0f}/sec")
    check("throughput > 2000/sec", batch_rate > 2000, f"{batch_rate:.0f}/sec")
    check("throughput > 5000/sec", batch_rate > 5000, f"{batch_rate:.0f}/sec")
    check("throughput > 10000/sec", batch_rate > 10000, f"{batch_rate:.0f}/sec")

    # ── Phase 3: Data Integrity ──
    print(f"\n=== Phase 3: Data Integrity ===")
    stats = get_job_stats()
    check("no pending jobs remain", stats.get("pending", -1) == 0, f"{stats.get('pending', '?')} pending")
    check("all jobs marked done", stats.get("done", 0) == created, f"{stats.get('done', 0)}/{created}")

    all_jobs = requests.get(f"{BASE}/jobs", timeout=30).json()
    ids = [j["id"] for j in all_jobs]
    check("no duplicate IDs", len(ids) == len(set(ids)), f"{len(ids)} total, {len(set(ids))} unique")
    statuses = set(j["status"] for j in all_jobs)
    check("no corrupted statuses", statuses.issubset({"pending", "done", "failed"}), f"found: {statuses}")

    # ── Phase 4: Concurrent Batch Race ──
    print(f"\n=== Phase 4: Concurrent Batch Race (500 jobs, 3 batchers) ===")
    for i in range(500):
        requests.post(f"{BASE}/jobs", json={"type": "send_email", "payload": f"race_{i}"}, timeout=10)

    batch_results = []
    race_lock = threading.Lock()

    def fire_batch():
        try:
            r = requests.post(f"{BASE}/jobs/batch", timeout=120)
            with race_lock:
                batch_results.append(r.json())
        except Exception as e:
            with race_lock:
                batch_results.append({"error": str(e)})

    threads = [threading.Thread(target=fire_batch) for _ in range(3)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    total_processed = sum(b.get("total", 0) for b in batch_results)
    print(f"  Batch results: {[b.get('total', 0) for b in batch_results]}")
    check("no double-processing", total_processed <= 500, f"processed {total_processed} from 500 jobs")

    race_stats = get_job_stats()
    check("no pending after race", race_stats.get("pending", -1) == 0, f"{race_stats.get('pending', '?')} pending")

    # ── Summary ──
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
