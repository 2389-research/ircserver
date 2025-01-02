# 🐍 Simple IRC Bot 🤖

Welcome to the Simple IRC Bot repository! This project aims to create a basic Internet Relay Chat (IRC) bot that listens for messages and responds to specific commands. 

## 📖 Summary of Project

This repository contains the entire codebase for developing a simple IRC bot using Python. The bot connects to an IRC server and can respond to messages sent in channels. It's specifically designed to be a minimalistic and easy-to-understand example of an IRC bot, perfect for anyone looking to learn about bot development or IRC protocols.

## 🚀 How to Use

To get your very own IRC bot up and running, follow these steps:

1. **Clone the Repository**
   ```bash
   git clone https://github.com/harperreed/simple-irc-bot.git
   cd simple-irc-bot
   ```

2. **Set Up the Environment**
   Ensure you have Python 3.12 or higher installed. You can check your Python version with:
   ```bash
   python --version
   ```

3. **Install Dependencies**
   Make sure to install the required dependencies. You can do this using `pip`:
   ```bash
   pip install -r requirements.txt
   ```

4. **Configure the Bot**
   Open `bot.py` and edit the `channel`, `nickname`, `server`, and `port` variables with your desired values.
   ```python
   channel = "#2389"
   nickname = "PingPongBot"
   server = "localhost"  # Example IRC server
   port = 6667
   ```

5. **Run the Bot**
   Use the following command to run your bot:
   ```bash
   python bot.py
   ```

   Once connected, you can type `!ping` in the channel to receive a `pong` response from the bot.

## 🛠️ Tech Info

This project uses the following technologies:

- **Python**: The programming language used to develop the bot.
- **`irc` library**: A Python library to handle the IRC protocol efficiently. It is specified as a dependency in `pyproject.toml`.

### Directory Structure

```
.
├── .python-version      # Python version used
├── bot.py               # Main bot code
└── pyproject.toml       # Project metadata and dependencies
```

### File Overview
- **`.python-version`**: Contains the required Python version.
- **`bot.py`**: The main implementation of the IRC bot.
- **`pyproject.toml`**: Project configuration file, including the project name, version, and dependencies.

For more information on the `irc` library, visit the [IRC lib documentation](https://irc.readthedocs.io/en/latest/).

---

Feel free to reach out if you have any questions or suggestions! Happy Coding! 🎉
