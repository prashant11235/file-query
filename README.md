Usage:

- run - `go run .`
- test - `go test -v`

________________________________________________________________________

Q: The .csv file could be very big (billions of entries) - how would your application perform? How would you optimize it?

Application performance for large files: 
- The current implementation loads the entire file in RAM before writing it to disk as well as stores the entire data in memory as well
  - If the uploaded file size is greater than (RAM2 size)/2 server will fail to process it with `fatal error: runtime: out of memory`

- The current implementation uses RWLock mutex to synchronize access - this means that while the data is being loaded in main memory all read requests are blocked
    - Frequent large files uploads will make the system unresponsive for large time periods

Key challenges with large-files:
- If file size > RAM memory then loading file to in-memory buffer and then writing is not possible - as currently implemented
  - Client side disconnection while uploading - leading to server rework if file reuploaded

- Loading entire data in main-memory not possible - as currently implemented

Solutions:
- File size > RAM memory
  - Client side chunking of file - divide file into small chunks (say 15 Mb)
  - Range header and 206 Partial Content response for chunk upload
  - Based on range header client only uploads remaining chunks
  - Store each chunk independently instead of a single-file for efficient query processing (details outlined below)
  - For client-side disconnection - Implement chunked upload with pause-resume - If client disconnects 


- Loading entire data in main-memory not possible
  - Instead of storing a single file - each chunk is stored independently in disk
  - We can now divide the GET call processing into 2 parts:
    - Identify the chunk which contains the queried identifier
    - Load the relevant chunk in main-memory from disk to return the details

________________________________________________________________________

Q: How would your application perform in peak periods (millions of requests per/minute)? How would you optimize it?

Performance (i.e. client side response latencies and server-throughput) degrades at high request load due to resource contention.

At the limit sever will drop requests with 503 status code.

For temporary spike in request load we can introduce a queue component - server picks requests from queue based on resource availability.

If the average request load is greater than server throughput we can:
- Optimize GET call processing
- Scaling - Vertical (at first) -> Horizontal

a. Optimizing GET call processing

Key idea: Small main-memory data-structure + Smart disk offload 

Approaches:
- Map-reduce paradigm - Need to store data in chunks - For each request create a fixed pool of goroutines and process a batch of chunks - Aggregate
  - Suitable for batch processing - not for online GET calls

- Bloom-Filter - To efficiently identify the chunk which contains the requested identifier.

We create a bloom-filter (a probabilistic data-structure for set membership) for each data-chunk

For an incoming GET request:
- Identify the chunk via set-membership query on per chunk bloom-filter  
- Once chunk is identified - we load the chunk data in main-memory map (~15 Mb) and search for the corresponding Id

```
func get_details(id string) Promotion {
    for chunk_id, filter in range chunk_bloom_filters:
    if filter.exists(id):
        disk_offload(chunk_id, id)

}

func disk_offload(chunk_id, id) Promotion {
    chunk_map = make(map[string]Promotion)
    // load chunk data from disk
    ...

    // find the required details from chunk map
    return chunk_map[id]
}
```

Memory size required:

- ~15 Mb file for 0.2 Million data-points => 75 MB file for 1 million data-points
- For 10 billion entries we have: file size = 10 * 1000 * 75 MB = 750 GB
- For 15 MB chunks - Number of chunks = 50,000
- For a false positive rate of 1% to store 0.2 Million entries we need ~250 KB per bloom filter.
- For 50K bloom-filters - this results in constant memory requirement of 1.25 GB (Optimal number of hash-functions = 7)

Query processing 
- Set membership check on 50K bloom-filters = 50K * O(k) where k = num hash-functions per filter = 0.35M operations
- Once chunk identified we load the chunk in-memory and search for corresponding id in O(1) via a map

Depending on difference between disk-seek time and main-memory size optimal parameters can be chosen.

b. Scaling

- Vertical scaling - We can utilize more powerful instances before going for horizontal scaling and adding distributed-system headaches!
- Horizontal scaling - As a last resort we can scale the system by adding additional nodes (details below)
________________________________________________________________________

Q: How would you operate this app in production (e.g. deployment, scaling, monitoring)

**Deployment** - Is quite subjective depending on context (cost/incentive structure) - cloud, on-prem or hybrid

General Objectives:
- System upgrades should be transparent to users
- Resilience - System should gracefuilly fail in case of high loads 
- Maintenance 

Options 
- Cloud v/s On-prem
- Tools - Docker, K8S, Standalone VM service

Upgrade Approaches:
- Blue-Green
- Rolling-upgrade

In general it is difficult to make choices in this space without additional context.

**Scaling** - esp. Horizontal scaling

To facilitate our service for horizontal scaling - we need to make it stateless.

Key idea: - separate read and write paths

Read-Replica pattern:
- A main node - that processes write/update promotion immutably
  - Need to replicate the file for all the nodes - Can be done via a background call from main node to replicas

- Replica + Main node - for processing read requests

NB: Read-replica approach makes sense over Leaderless since it seems that read requests >> update requests - If this assumption is incorrect we may need to refine the design further

Once the service is stateless we can spin up multiple nodes and deploy our service on each node with a load-balancer in front acting as proxy.

- If the data-size is too large we may also need to consider sharding - viz. partition data and make individual nodes responsible for a subset of data.
  - Need to add consistent hashing to limit key redistribution in case of node unavailability


**Monitoring**

Which Stats to monitor?
- Application - Load (requests/second), Responses (2xx, 4xx, 5xx), Latencies (median, p95, p99)
- Infra - CPU usage, Memory consumption, Disk seek times etc.

General ideas:
- Metrics exporter component (running per node) - Push vs Pull
- Metrics storage - Dedicated time-series database like Prometheus
- Graphing/Alerting component