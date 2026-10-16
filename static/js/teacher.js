

function addSubject() {
    const subjects = document.getElementById("subjects");

    const row = document.createElement("div");
    row.className = "subject-row";

    const input = document.createElement("input");
    input.type = "text";
    input.name = "subject";
    input.placeholder = "نام درس دیگر";
    input.required = true;

    const removeButton = document.createElement("button");
    removeButton.type = "button";
    removeButton.textContent = "حذف درس";

    removeButton.onclick = function () {
        row.remove();
    };

    row.appendChild(input);
    row.appendChild(removeButton);
    subjects.appendChild(row);
}