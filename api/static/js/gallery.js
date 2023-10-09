let picks = {
    count: 0,
    picks: []
}

function createSecureCookie(name, value, expirationDays, path = "/", domain = "") {

    // Calculate the expiration date
    const currentDate = new Date();
    currentDate.setTime(currentDate.getTime() + (expirationDays * 24 * 60 * 60 * 1000));
    const expires = `expires=${currentDate.toUTCString()}`;

    // Set the Secure attribute to ensure the cookie is secure (HTTPS only)
    const secureFlag = "Secure";

    // Construct the cookie string
    const cookieString = `${name}=${value}; ${expires}; path=${path}; domain=${domain}; ${secureFlag}`;

    // Set the cookie
    document.cookie = cookieString;

}
function setCookie(cookieName, cookieValue, expirationDays) {
    const expirationDate = new Date();
    expirationDate.setDate(expirationDate.getDate() + expirationDays);

    const cookieString = `${encodeURIComponent(cookieName)}=${encodeURIComponent(
        cookieValue
    )}; expires=${expirationDate.toUTCString()}; path=/`;

    document.cookie = cookieString;
}

function savePicksToCookie(value){
    //createSecureCookie("picks",value,"7")
    setCookie("picks",JSON.stringify(value),7)

}

function get_picks(url){

    let xhr = new XMLHttpRequest();
    xhr.open("GET", url);
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            if (xhr.status === 200) {
                return xhr.responseText
            } else {
                console.log("Something went wrong getting picks from the server")
                alert("Something went wrong getting picks from the server")
                return("error")
            }
        }
    };
    xhr.send();
}

function markImage(id) {
    let img = document.getElementById(id)
    if (img.alt === "1") {
        img.alt = "0";
        img.childNodes[0].style = null
        window.picks.count--
        window.picks.picks = window.picks.picks.filter(item => item !== id) // Removes the picture from picks list
        savePicksToCookie(window.picks)
        document.getElementById("counter").innerHTML = window.picks.count + " Items Selected"
    } else {
        img.alt = "1"
        let borderPX = Math.floor(img.childNodes[0].width * .0125)
        img.childNodes[0].style = "outline: " + borderPX + "px solid #ff6600;outline-offset: -" + borderPX + "px;"
        window.picks.count++
        window.picks.picks.push(id) // Adds a picture to the list
        savePicksToCookie(window.picks)
        document.getElementById("counter").innerHTML = window.picks.count + " Items Selected"
    }
    document.getElementById("save_status").innerHTML = ""
}

function save() {

    let picks = getCookie("picks")

    let xhr = new XMLHttpRequest();
    xhr.open("POST", window.location.href + "/savePicks");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            console.log(xhr.status);
            console.log(xhr.responseText);
            if (xhr.status === 200 || xhr.status === 0) {
                document.getElementById("save_status").innerHTML = "Saved!"
            } else {
                alert("Something went wrong saving your selections")
                alert(xhr.status)
            }
        }
    };
    console.log(picks)
    xhr.send(picks);

}

function getCookie(cookieName) {
    const name = cookieName + "=";
    const decodedCookie = decodeURIComponent(document.cookie);
    const cookieArray = decodedCookie.split(';');

    for (let i = 0; i < cookieArray.length; i++) {
        let cookie = cookieArray[i].trim();
        if (cookie.indexOf(name) === 0) {
            return cookie.substring(name.length, cookie.length);
        }
    }

    // If the cookie doesn't exist, return null
    return null;
}


function loadSelected(){

    // Update the picks cookie
    let url = window.location.href;
    url = url.split("/");
    url[5] = String("updatePicksCookie")
    url = url.join("/")
    get_picks(url) // Updates the cookie

    // Set picks to cookie value
    let picks = getCookie("picks")
    picks = JSON.parse(picks)
    if (picks.count === 0){
        picks = {
            count: 0,
            picks: []
        }
    }
    window.picks = picks
    savePicksToCookie(window.picks)

    document.getElementById("counter").innerHTML = window.picks.count + " Items Selected"


    for (let i=0;i<=window.picks.picks.length-1;i++) {

        try {
            let id = window.picks.picks[i]
            let img = document.getElementById(id)
            img.alt = "1"
            let borderPX = Math.floor(img.childNodes[0].width * .0125)
            img.childNodes[0].style = "outline: " + borderPX + "px solid #ff6600;outline-offset: -" + borderPX + "px;"
        } catch {} // Do nothing on error


    }
}

// Wait for all images to load
window.addEventListener("load", function () {

    // Hide the loading screen once all images are loaded
    const loadingScreen = document.getElementById("loading-screen");
    loadingScreen.style.display = "none";

    loadSelected() // Mark the previously selected images

    let url = window.location.href;

    console.log(JSON.stringify(window.picks) + " final")

    url = url.split("/");
    document.getElementById("page_num").innerHTML = "Page " + String(parseInt(url[url.length - 1]) + 1)
});

function nextPage() {
    save()
    let url = window.location.href;
    url = url.split("/");
    url[url.length - 1] = String(parseInt(url[url.length - 1]) + 1)
    console.log(url.join("/"))
    window.location.href = url.join("/")
}

function previousPage() {
    save()
    let url = window.location.href;
    url = url.split("/");
    if ((parseInt(url[url.length - 1]) - 1) >= 0) {
        url[url.length - 1] = String(parseInt(url[url.length - 1]) - 1)
        console.log(url.join("/"))
        window.location.href = url.join("/")
    }
}
