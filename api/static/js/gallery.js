selected_counter = 0 //Used to store the number of selected photos

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


function get_picks(url,shoot,auth){

    let xhr = new XMLHttpRequest();
    xhr.open("GET", url + "/getPicks");
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            console.log(xhr.status);
            console.log(xhr.responseText);
            if (xhr.status === 200) {
                return xhr.responseText
            } else {
                console.log("Something went wrong getting picks from the server")
                alert("Something went wrong getting picks from the server")
                return("error")
            }
        }
    };
    return "error"
}


function markImage(id) {
    var img = document.getElementById(id)
    if (img.alt == "1") {
        img.alt = "0";
        img.childNodes[0].style = null
        selected_counter--
        document.getElementById("counter").innerHTML = selected_counter + " Items Selected"
    } else {
        img.alt = "1"
        borderPX = Math.floor(img.childNodes[0].width * .0125)
        img.childNodes[0].style = "outline: " + borderPX + "px solid #ff6600;outline-offset: -" + borderPX + "px;"
        selected_counter++
        document.getElementById("counter").innerHTML = selected_counter + " Items Selected"
    }
    document.getElementById("save_status").innerHTML = ""
}

function save() {
    var pics = {
        picks: [],
        count: 0
    }
    var gallery = document.getElementById("gallery")
    gallery = gallery.childNodes

    for (var i = 0; i < gallery.length; i++) {

        if (gallery[i].id != undefined) {

            if (gallery[i].alt == "1") {

                pics.picks.push(gallery[i].id)

            }
        }
    }

    pics.count = pics.picks.length

    let xhr = new XMLHttpRequest();
    xhr.open("POST", window.location.href + "/submitPicks");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            console.log(xhr.status);
            console.log(xhr.responseText);
            if (xhr.status === 200) {
                document.getElementById("save_status").innerHTML = "Saved!"
            } else {
                alert("Something went wrong submitting your selections")
            }
        }
    };

    xhr.send(JSON.stringify(pics));

}

// Wait for all images to load
window.addEventListener("load", function () {
    // Hide the loading screen once all images are loaded
    const loadingScreen = document.getElementById("loading-screen");
    loadingScreen.style.display = "none";

    let url = window.location.href;

    let picks = get_picks(url)
    createSecureCookie("picks",picks,5)
    console.log("Test")


    url = url.split("/");
    document.getElementById("page_num").innerHTML = "Page " + String(parseInt(url[url.length - 1]) + 1)
});

function nextPage() {
    let url = window.location.href;
    url = url.split("/");
    url[url.length - 1] = String(parseInt(url[url.length - 1]) + 1)
    console.log(url.join("/"))
    window.location.href = url.join("/")
}

function previousPage() {
    var url = window.location.href;
    url = url.split("/");
    if ((parseInt(url[url.length - 1]) - 1) >= 0) {
        url[url.length - 1] = String(parseInt(url[url.length - 1]) - 1)
        console.log(url.join("/"))
        window.location.href = url.join("/")
    }
}

