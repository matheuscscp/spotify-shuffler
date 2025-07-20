package main

const authSuccessPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Authentication Complete</title>
  <style>
    /* Make the body take the full viewport height, remove default margins */
    html, body {
      height: 100%;
      margin: 0;
    }
    /* Center contents */
    body {
      display: flex;
      justify-content: center; /* horizontal */
      align-items: center;    /* vertical */
      text-align: center;     /* center text inside */
      font-family: Arial, sans-serif;
    }
    p {
      font-size: 1.2em;
    }
  </style>
</head>
<body>
  <p>
    Authentication successful!<br>
    This browser tab will close itself in <span id="count">10</span> seconds.
  </p>

  <script>
    (function() {
      var seconds = 10;
      var el = document.getElementById("count");

      var timer = setInterval(function() {
        seconds--;
        if (seconds <= 0) {
          clearInterval(timer);
          // Attempt to close
          window.close();
          // Fallback
          window.open('', '_self');
          window.close();
        } else {
          el.textContent = seconds;
        }
      }, 1000);
    })();
  </script>
</body>
</html>`
