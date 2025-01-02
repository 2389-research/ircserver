import irc.bot
import irc.strings

class SimpleBot(irc.bot.SingleServerIRCBot):
    def __init__(self, channel, nickname, server, port=6667):
        irc.bot.SingleServerIRCBot.__init__(self, [(server, port)], nickname, nickname)
        self.channel = channel

    def on_welcome(self, connection, event):
        """Called when bot connects to the server"""
        connection.join(self.channel)
        print(f"Connected to {self.channel}")

    def on_pubmsg(self, connection, event):
        """Called when a message is received"""
        message = event.arguments[0]
        
        # Check if message is !ping
        if message == "!ping":
            connection.privmsg(self.channel, "pong")

def main():
    # Bot configuration
    channel = "#2389"
    nickname = "PingPongBot"
    server = "localhost"  # Example IRC server
    port = 6667

    # Create and start the bot
    bot = SimpleBot(channel, nickname, server, port)
    bot.start()

if __name__ == "__main__":
    main()
