function markImage(id) {
    img = document.getElementById(id)
    if (img.alt == "1") {
        img.alt = "0";
        img.childNodes[0].style = null
    } else {
        img.alt = "1"
        borderPX = Math.floor(img.childNodes[0].width * .025)
        img.childNodes[0].style = "outline: " + borderPX + "px solid #51f542;outline-offset: -" + borderPX + "px;"
    }
}
