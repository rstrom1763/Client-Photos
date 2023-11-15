function goToShoot(passedDiv){


    let shootName = passedDiv.querySelector('#name');
    shootName = shootName.innerHTML;

    let url = window.location.href;
    url = url.split("/");

    url[3] = "shoot"
    url[4] = shootName
    url[5] = "0"

    console.log(url)
    window.location.href = url.join("/")

}