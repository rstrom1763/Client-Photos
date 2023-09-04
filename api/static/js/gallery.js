selected_counter = 0 //Used to store the number of selected photos

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
}

function submit() {
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
    xhr.open("POST", window.location.href+"/submitPicks");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            console.log(xhr.status);
            console.log(xhr.responseText);
            if (xhr.status === 200) {
                alert("Your selections have been submitted")
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
});
