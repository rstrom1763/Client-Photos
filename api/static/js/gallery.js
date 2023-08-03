function markImage(id) {
    var img = document.getElementById(id)
    if (img.alt == "1") {
        img.alt = "0";
        img.childNodes[0].style = null
    } else {
        img.alt = "1"
        borderPX = Math.floor(img.childNodes[0].width * .0125)
        img.childNodes[0].style = "outline: " + borderPX + "px solid #ff6600;outline-offset: -" + borderPX + "px;"
    }
}

function submit() {
    var pics = {
        picks: []
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

    let xhr = new XMLHttpRequest();
    xhr.open("POST", window.location.href + "submit");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            console.log(xhr.status);
            console.log(xhr.responseText);
            alert("Your selections have been submitted")
        }
    };

    xhr.send(JSON.stringify(pics));

}

// Wait for all images to load
window.addEventListener("load", function () {
    // Hide the loading screen once all images are loaded
    const loadingScreen = document.getElementById("loading-screen");
    loadingScreen.style.display = "none";
});