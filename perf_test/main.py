import sys
import time
import uuid

import requests

BASE = "http://localhost:8080"

PASS = "\033[92mPASS\033[0m"
FAIL = "\033[91mFAIL\033[0m"

results: list[tuple[str, bool, str]] = []


def check(label: str, passed: bool, detail: str = "") -> bool:
    results.append((label, passed, detail))
    status = PASS if passed else FAIL
    print(f"  [{status}] {label}{' — ' + detail if detail else ''}")
    return passed


def elapsed(fn) -> tuple[float, any]:
    t = time.perf_counter()
    r = fn()
    return time.perf_counter() - t, r


def create_job(job_type: str = "send_email", payload: str = "test@example.com") -> requests.Response:
    return requests.post(f"{BASE}/jobs", json={"type": job_type, "payload": payload})


def pending_jobs(all_jobs: list[dict]) -> list[dict]:
    return [j for j in all_jobs if j["status"] == "pending"]


def poll_until_done(job_ids: list[str], timeout: float = 5.0) -> tuple[bool, float]:
    deadline = time.perf_counter() + timeout
    start = time.perf_counter()
    while time.perf_counter() < deadline:
        statuses = {j["id"]: j["status"] for j in requests.get(f"{BASE}/jobs").json()}
        if all(statuses.get(jid) in ("done", "failed") for jid in job_ids):
            return True, time.perf_counter() - start
        time.sleep(0.05)
    return False, timeout


def test_easy():
    print("\n=== LEVEL 1: HTTP Server ===")

    delta, r = elapsed(lambda: create_job())
    check("POST /jobs returns 201", r.status_code == 201, f"{delta*1000:.0f}ms")
    check("POST /jobs latency < 100ms", delta < 0.1, f"{delta*1000:.0f}ms")

    job = r.json()
    check("response has id, type, payload, status", all(k in job for k in ("id", "type", "payload", "status")))
    check("status is pending", job.get("status") == "pending")

    delta, r = elapsed(lambda: requests.get(f"{BASE}/jobs/{job['id']}"))
    check("GET /jobs/{id} returns 200", r.status_code == 200, f"{delta*1000:.0f}ms")
    check("GET /jobs/{id} latency < 100ms", delta < 0.1, f"{delta*1000:.0f}ms")

    delta, r = elapsed(lambda: requests.get(f"{BASE}/jobs"))
    check("GET /jobs returns 200", r.status_code == 200, f"{delta*1000:.0f}ms")
    check("GET /jobs returns list", isinstance(r.json(), list))

    delta, r = elapsed(lambda: requests.post(f"{BASE}/jobs", data="not json", headers={"Content-Type": "application/json"}))
    check("POST /jobs malformed JSON → 400", r.status_code == 400)



def test_hard():
    print("\n=== LEVEL 3: DB, WaitGroups & Validation ===")

    r = create_job(job_type="invalid_type")
    check("invalid type → 422", r.status_code == 422, f"got {r.status_code}")

    r = create_job(payload="")
    check("empty payload → 422", r.status_code == 422, f"got {r.status_code}")

    r = create_job(payload="x" * 501)
    check("payload > 500 chars → 422", r.status_code == 422, f"got {r.status_code}")

    job_ids = [create_job(job_type=t).json()["id"] for t in ["resize_image", "send_email", "generate_report"] * 3]

    delta, r = elapsed(lambda: requests.post(f"{BASE}/jobs/batch"))
    check("POST /jobs/batch returns 200", r.status_code == 200, f"{delta*1000:.0f}ms")

    body = r.json()
    check("summary has total, succeeded, failed", all(k in body for k in ("total", "succeeded", "failed")))
    check("total = succeeded + failed", body.get("total") == body.get("succeeded", 0) + body.get("failed", 0))
    check("total matches dispatched job count", body.get("total") == len(job_ids), f"expected {len(job_ids)}, got {body.get('total')}")

    max_expected = (len(job_ids) / 3 + 1) * 0.4
    check(
        f"batch blocks and finishes in < {max_expected*1000:.0f}ms",
        delta < max_expected,
        f"actual {delta*1000:.0f}ms",
    )

    all_jobs = {j["id"]: j for j in requests.get(f"{BASE}/jobs").json()}
    check(
        "all batch jobs marked done after response",
        all(all_jobs.get(jid, {}).get("status") == "done" for jid in job_ids),
    )
    check("summary reports 0 failed", body.get("failed") == 0, f"got {body.get('failed')}")


def main():
    try:
        requests.get(BASE, timeout=2)
    except Exception:
        print(f"Server not reachable at {BASE}. Start it with: go run .")
        sys.exit(1)

    test_easy()
    requests.post(f"{BASE}/jobs/batch")
    test_hard()

    total = len(results)
    passed = sum(1 for _, ok, _ in results if ok)
    failed = total - passed

    print(f"\n{'='*40}")
    print(f"Results: {passed}/{total} passed", end="")
    if failed:
        print(f"  ({failed} failed)")
    else:
        print("  — all good")

    sys.exit(0 if failed == 0 else 1)


if __name__ == "__main__":
    main()
