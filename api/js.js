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
    var pics = []
    var gallery = document.getElementById("gallery")
    gallery = gallery.childNodes

    for (var i = 0; i < gallery.length; i++) {

        if (gallery[i].id != undefined) {

            if (gallery[i].alt == "1") {

                pics.push(gallery[i].id)

            }
        }
    }

    let xhr = new XMLHttpRequest();
    xhr.open("POST", "https://pastromiphotography.com/submit");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            console.log(xhr.status);
            console.log(xhr.responseText);
            alert("Your selections have been submitted!!!")
        }
    };

    xhr.send(JSON.stringify(pics));

}