document.addEventListener('DOMContentLoaded', function() {
    // Update dashboard data every 2 seconds
    updateDashboard();
    setInterval(updateDashboard, 2000);

    // Handle message form submission
    document.getElementById('message-form').addEventListener('submit', function(e) {
        e.preventDefault();
        
        const target = document.getElementById('target').value;
        const content = document.getElementById('content').value;

        fetch('/api/send', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ target, content })
        })
        .then(response => {
            if (response.ok) {
                document.getElementById('content').value = '';
            }
        })
        .catch(error => console.error('Error:', error));
    });
});

function updateDashboard() {
    fetch('/api/data')
        .then(response => response.json())
        .then(data => {
            updateUsersList(data.users);
            updateChannelsList(data.channels);
        })
        .catch(error => console.error('Error:', error));
}

function updateUsersList(users) {
    const usersList = document.getElementById('users-list');
    usersList.innerHTML = users.map(user => `
        <div class="user">
            <strong>${user.nickname}</strong> (${user.username})
            <div class="channels">In: ${user.channels.join(', ') || 'No channels'}</div>
        </div>
    `).join('');
}

function updateChannelsList(channels) {
    const channelsList = document.getElementById('channels-list');
    channelsList.innerHTML = channels.map(channel => `
        <div class="channel">
            <strong>${channel.name}</strong>
            <div class="topic">${channel.topic || 'No topic'}</div>
            <div class="users">Users (${channel.userCount}): ${channel.users.join(', ')}</div>
        </div>
    `).join('');
}
