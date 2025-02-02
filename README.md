# Bluengo Simple Worker
Poor's man, yet effective, job queue/worker. Should run fine as is in modern Linux releases.

**See our office webcam at [https://www.bluengo.com](https://www.bluengo.com)**

Bluengo Simple Worker is a lightweight distributed job processing system written in Go. It consists of three subcommands:

- **server**: Runs the HTTP job server.
- **worker**: Runs a worker that polls the server for jobs and executes them.
- **add**: Adds a new job to the server's job queue.

## Table of Contents

- [Usage](#usage)
  - [Server](#server)
  - [Worker](#worker)
  - [Add Job](#add-job)
- [Examples](#examples)
- [Debug Mode](#debug-mode)


## Usage

The program uses subcommands to start the server, worker, or add jobs. The general usage is:

```bash
./bluengo <subcommand> [flags...]
```

### Server

Start the job server which listens for incoming job requests and job polls from workers.

**Usage:**

```bash
./bluengo server [flags]
```

**Flags:**

- `-port`  
  Port for the server (default: `8080`).

- `-password`  
  Password for authenticating requests. If set, workers and job submissions must include this password in the `X-Job-Password` header.

- `-debug`  
  Enable debug logging (default: `false`).

**Example:**

```bash
./bluengo server -port 8080 -password mysecret -debug
```

### Worker

Run a worker that polls the job server and executes jobs concurrently.

**Usage:**

```bash
./bluengo worker [flags]
```

**Flags:**

- `-server`  
  URL of the job server (default: `http://localhost:8080`).

- `-slots`  
  Number of concurrent job slots (default: number of CPU cores).

- `-poll`  
  Poll interval in seconds (default: `1`).

- `-password`  
  Password for authenticating with the server.

- `-debug`  
  Enable debug logging (default: `false`).

**Example:**

```bash
./bluengo worker -server http://localhost:8080 -slots 4 -poll 2 -password mysecret -debug
```

### Add Job

Add a new job to the job server's queue.

**Usage:**

```bash
./bluengo add [flags]
```

**Flags:**

- `-server`  
  URL of the job server (default: `http://localhost:8080`).

- `-cmd`  
  The shell command to execute (required).

- `-timeout`  
  Timeout in seconds for the job execution (default: `10`).

- `-password`  
  Password for authenticating with the server.

- `-debug`  
  Enable debug logging (default: `false`).

**Example:**

```bash
./bluengo add -server http://localhost:8080 -cmd "echo 'Hello, World!'" -timeout 5 -password mysecret -debug
```

## Examples

1. **Start the server:**

   ```bash
   ./bluengo server -port 8080 -password mysecret
   ```

2. **Start a worker:**

   ```bash
   ./bluengo worker -server http://localhost:8080 -slots 4 -poll 2 -password mysecret
   ```

3. **Submit a job:**

   ```bash
   ./bluengo add -server http://localhost:8080 -cmd "sleep 10" -timeout 15 -password mysecret
   ```

## Debug Mode

To enable verbose logging for troubleshooting, add the `-debug` flag to any subcommand:

```bash
./bluengo worker -server http://localhost:8080 -slots 4 -debug
```

## License

This project is a proprietary project.
