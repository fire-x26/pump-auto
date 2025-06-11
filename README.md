# Pump.fun Auto Sniper (Go Version)

A robot that automatically snipes new tokens on the Pump.fun platform.

## Project Structure

- **/cmd**: Main application(s)
- **/internal**: Private application code.
  - **/bot**: Core bot logic, including event listeners and trading functions.
  - **/analyzer**: Custom token filtering, currently only supports filtering website and twitter.
  - **/filter**: Token filtering logic.
  - **/execctor**: Send trades based on stop-loss and take-profit conditions.

## Prerequisites

- Go (version 1.20 or higher recommended)
- Solana account with SOL for transactions
- RPC endpoint for Solana

## Setup (Placeholder)

1.  **Clone the repository:**
    ```bash
    git clone <repository-url> pump_auto
    cd pump_auto
    ```
2.  **Configuration:**
    ```
    cd txSend
    add code:
      const PRIVATE_KEY = ""
      const RPC_URL = ""
    ```
4.  **Install dependencies:**
    ```bash
    go mod tidy
    ```
5.  **Run the bot:**
    ```bash
    go run cmd/main.go
    ```

## Disclaimer

This is a work in progress. Trading cryptocurrencies involves significant risk. Use this software at your own risk. 
