# Locust Load-Test Results

Target: AWS ECS Fargate Product API in `us-east-1`

Request mix: 3 GET tasks for every 1 POST task (approximately 75% reads and 25% writes).

All tests used one local Locust process. The 50-user comparison used the same user count, spawn rate, run time, target, request body, and 1–2 second wait time for both client types.

## Test Parameters

| Test | Client | Users | Spawn rate | Duration |
| --- | --- | ---: | ---: | ---: |
| Comparison 1 | `HttpUser` | 50 | 10 users/s | 60 s |
| Comparison 2 | `FastHttpUser` | 50 | 10 users/s | 60 s |
| Stress stage 1 | `FastHttpUser` | 200 | 40 users/s | 45 s |
| Stress stage 2 | `FastHttpUser` | 500 | 100 users/s | 45 s |

## Results

Values below come from the completed Locust console and HTML reports.

| Test | Requests | Failures | Requests/s | Average | Median | p95 | p98 | Maximum |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| HttpUser, 50 users | 1,824 | 0 | 31.86 | 32.73 ms | 27 ms | 40 ms | 77 ms | 1,103 ms |
| FastHttpUser, 50 users | 1,907 | 0 | 31.94 | 29.55 ms | 26 ms | 47 ms | 79 ms | 269 ms |
| FastHttpUser, 200 users | 5,661 | 0 | 126.54 | 32 ms | 25 ms | 41 ms | 180 ms | 314 ms |
| FastHttpUser, 500 users | 14,020 | 0 | 313.35 | 53.85 ms | 26 ms | 77 ms | 720 ms | 2,418 ms |

## HttpUser vs. FastHttpUser

At 50 users, throughput was almost identical: 31.87 requests/s for `HttpUser` and 31.98 requests/s for `FastHttpUser`. The 1–2 second wait time intentionally models user think time and limits how often each user sends a request, so the load generator's HTTP implementation was not the throughput bottleneck in this experiment.

`FastHttpUser` had a slightly lower average and median response time and a much lower maximum in this run. It uses `geventhttpclient`, which has lower client-side overhead than the Python `requests`-based `HttpUser`. One run is not enough to claim that every latency difference is caused by the client library, because Internet and ECS latency also vary between samples.

## Stress Point

The service stayed healthy through 200 users, producing about 126 requests/s with zero failures and a 41 ms p95.

At 500 users, throughput increased to about 313 requests/s and there were still no failed requests, but the latency distribution widened sharply. Average latency rose to about 54 ms, p95 rose to 74 ms, p98 reached 720 ms, and the maximum reached approximately 2.4 seconds. This is the clearest point at which the single small Fargate task began to struggle. A system can be overloaded before it returns errors; rapidly increasing tail latency is an early warning.

The API returned `200 OK` immediately after the test, confirming that it recovered after the load was removed.

## GET and POST Tradeoff

Product reads are expected to be more common than product updates, which is why the workload uses a 3:1 GET-to-POST ratio. The server's `sync.RWMutex` allows multiple GET handlers to read the map concurrently. A POST requires the exclusive write lock, temporarily blocking both readers and other writers. Because every request targets the same product and one quarter of requests are writes, lock contention can contribute to tail latency at high concurrency.

Other likely limits include the task's 0.25 vCPU and 512 MiB configuration, network latency between the local load generator and AWS, and the single ECS task count.

## Saved Reports

- `httpuser_50.html` and `httpuser_50_*.csv`
- `fasthttpuser_50.html` and `fasthttpuser_50_*.csv`
- `stress_fast_200.html` and `stress_fast_200_*.csv`
- `stress_fast_500.html` and `stress_fast_500_*.csv`

Open the HTML reports in a browser to capture the required screenshots. The comparison screenshots should show the 50-user `HttpUser` and `FastHttpUser` reports. The 500-user report should be used as the stress-test screenshot.
