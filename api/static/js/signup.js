function submit_new_user(){
    var user = {};
    user.username = document.getElementById("username").value;
    user.password = document.getElementById("password").value;
    user.email = document.getElementById("email").value;
    user.first = document.getElementById("first name").value;
    user.last = document.getElementById("last name").value;
    user.address = document.getElementById("address").value;
    user.city = document.getElementById("city").value;
    user.state = document.getElementById("state").value;
    user.zip = document.getElementById("zip").value;
    user.phone = document.getElementById("phone number").value;

    let currentLink = window.location.href;
    console.log(window.location.href.substring(0,currentLink.lastIndexOf("/")) + "/createUser")
    let xhr = new XMLHttpRequest();
    xhr.open("POST", window.location.href.substring(0,currentLink.lastIndexOf("/")) + "/createUser");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.setRequestHeader("Content-Type", "application/json");

    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            console.log(xhr.status);
            console.log(xhr.responseText);
            if (xhr.status === 200){
                alert("User Created!")
            } else{
                alert("Something went wrong :(")
            }
        }
    };
    xhr.send(JSON.stringify(user));

    
    let newLink = currentLink.substring(0,currentLink.lastIndexOf("/"))
    newLink += "/login"
    window.location.href = newLink
}