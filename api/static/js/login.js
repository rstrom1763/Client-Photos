// Set a cookie with a name, value, and optional attributes
function setCookie(name, value, minsToExpire) {
    minsToExpire = minsToExpire / 1440 // Convert mins to days
    let cookieString = `${encodeURIComponent(name)}=${encodeURIComponent(value)}`;

    if (minsToExpire) {
        const expirationDate = new Date();
        expirationDate.setDate(expirationDate.getDate() + minsToExpire);
        cookieString += `; expires=${expirationDate.toUTCString()}`;
    }

    document.cookie = cookieString;
}



function login() {
    var auth = {};
    auth.username = document.getElementById("username").value;
    auth.password = document.getElementById("password").value;

    let currentLink = window.location.href;
    console.log(window.location.href.substring(0, currentLink.lastIndexOf("/")) + "/signin")
    let xhr = new XMLHttpRequest();
    xhr.open("POST", window.location.href.substring(0, currentLink.lastIndexOf("/")) + "/signin");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            console.log(xhr.status);
            console.log(xhr.responseText);
            if (xhr.status === 200 || xhr.status === 202) {
                console.log("Authentication success!")
                let newLink = currentLink.substring(0, currentLink.lastIndexOf("/"))
                newLink += "/home"
                window.location.href = newLink
            } else if (xhr.status === 404) {
                alert(xhr.responseText)
            } else {
                alert("Something went wrong")
                console.log("Something went wrong")
            }
        }
    };
    xhr.send(JSON.stringify(auth));
}