## Multi-Currency Command-Line Cryptocurrency Wallet

A powerful, non-custodial CLI wallet built for developers and advanced users. Securely manage your cryptocurrency portfolio — including BTC, ETH, and major ERC-20 tokens — directly from your terminal with full control, security, and scriptability.

### Key Features

🚀 Multi-Currency Support

One unified wallet for Bitcoin, Ethereum, and major token standards — manage multiple chains seamlessly from a single interface.

⚡ Lightweight & Private

No bloated GUI. No unnecessary background services. Runs efficiently on minimal resources and avoids reliance on centralized infrastructure.

🔎 Transparent & Auditable

Fully open-source. Inspect the code, verify every operation, and maintain complete control over your funds at all times.

## Usage

### Wallet Initialization

Before using the wallet, you need to initialize it with your mnemonic seed phrase. Use the `create` command to set up your wallet:

```bash
./easywallet -mode=create
```

This will prompt you to:

1. **Enter your mnemonic** - Your 12 or 24-word seed phrase
2. **Enter seed encryption password** - An optional password to encrypt your seed file

Example:
```
Enter mnemonic: word1 word2 word3 ... word12
Enter seed encryption password: your_secure_password
```

> ⚠️ **Warning**: If you leave the encryption password empty, your seed will be stored unencrypted. This is a security risk!

The wallet creates a `seed.dat` file in the same directory containing your encrypted master seed.

### Interactive Mode

For a user-friendly experience, run the wallet in interactive mode:

```bash
./easywallet -mode=i
```

If your seed is encrypted, you'll be prompted for the encryption password first:

```
Enter seed encryption password: your_secure_password
```

Once authenticated, you'll see the main menu:

```
Enter command: 
1: get balance
2: send transaction
3: exit
```

#### Checking Balance (Option 1)

1. Select option `1` for get balance
2. Enter the coin symbol (e.g., `btc`, `eth`, `usdt`)

Example output:
```
Enter coin: eth
Coin:  eth
Current address:  0x742d35Cc6634C0532925a3b844Bc9e7595f2bD5e
Balance:  1.5
```

#### Sending Transactions (Option 2)

1. Select option `2` for send transaction
2. Enter the coin symbol
3. Enter the recipient's address
4. Enter the amount to send

Example:
```
Enter coin: eth
Enter recipient address: 0x8Ba1f109551bD432803012645Ac136ddd64DBA72
Enter amount: 0.1
Current address:  0x742d35Cc6634C0532925a3b844Bc9e7595f2bD5e
Balance:  1.5
Coin:  eth
Address:  0x8Ba1f109551bD432803012645Ac136ddd64DBA72
Tx:  0xabc123...
```

#### Exit (Option 3)

Select option `3` or any other number to exit the interactive mode.

### Command-Line Mode (Non-Interactive)

For scripting and automation, you can run commands directly without interactive mode:

#### Check Balance
```bash
./easywallet -password=your_password -command=balance -coin=eth
```

#### Send Transaction
```bash
./easywallet -password=your_password -command=sendtx -coin=eth -address=0xRecipient -amount=0.1
```

### Available Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| `-mode` | `create` for initialization, `i` for interactive mode | No |
| `-password` | Seed encryption password | If seed is encrypted |
| `-command` | `balance` or `sendtx` | Yes (non-interactive) |
| `-coin` | Coin symbol (btc, eth, usdt, etc.) | Yes |
| `-address` | Recipient address (for sendtx) | For sendtx |
| `-amount` | Amount to send (for sendtx) | For sendtx |
