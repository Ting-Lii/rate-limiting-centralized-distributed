import time
import random
import requests

SERVERS = [
    "http://localhost:8081",
    "http://localhost:8082",
    "http://localhost:8083",
    "http://localhost:8084",
    "http://localhost:8085",
]
# USER_ID = "userA"

def pick_server_uniform():
    return random.choice(SERVERS)

def pick_server_skewed():
    r = random.random()
    if r < 0.8:
        return SERVERS[0]           # 80% to server1
    else:
        return random.choice(SERVERS[1:])  # server2-5 each one gets around 20%

def run_test(user_id, pick_server_fn, duration_sec=60):
    start = time.time()
    ok = 0
    blocked = 0
    allowed_table = {SERVERS[0]: 0, SERVERS[1]: 0 , SERVERS[2]: 0, SERVERS[3]: 0, SERVERS[4]: 0}
    block_table = {SERVERS[0]: 0, SERVERS[1]: 0 , SERVERS[2]: 0, SERVERS[3]: 0, SERVERS[4]: 0}
    while time.time() - start < duration_sec:
        base = pick_server_fn()
        try:
            res = requests.get(
                f"{base}/test",
                headers={"X-User-ID": user_id},
                timeout=1.0,
            )
            if res.status_code == 200:
                ok += 1
                allowed_table[base] += 1
            elif res.status_code == 429:
                blocked += 1
                block_table[base] += 1
        except Exception:
            pass
    for s in allowed_table:
        print("Server:", s, "Allowed:", allowed_table[s], "Blocked:", block_table[s])
    return ok, blocked

print("=== Distributed local limiter / uniform ===")
ok_u, block_u = run_test("userA", pick_server_uniform)
print("allowed:", ok_u, "blocked:", block_u)

print("=== Distributed local limiter / skewed ===")
ok_s, block_s = run_test("userB", pick_server_skewed)
print("allowed:", ok_s, "blocked:", block_s)
