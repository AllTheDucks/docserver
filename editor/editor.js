var editor = ace.edit("editor");
editor.getSession().setMode("ace/mode/markdown");

var saveButton = $("#saveButton");

saveButton.click(function (e){
	saveFile(e);
});

$(window).bind('keydown', function(event) {
    if (event.ctrlKey || event.metaKey) {
        switch (String.fromCharCode(event.which).toLowerCase()) {
        case 's':
            event.preventDefault();
            saveFile(event)
            break;

        }
    }
});

function saveFile(e) {
	$("#saveSpinner").show();
	$.post("", editor.getValue())
		.done(function () {
			switchClass($("#saveStatus"), 'failMsg', 'successMsg');
			$("#saveStatus").text("Last Saved: " + new Date());
		})
		.fail(function () {
			switchClass($("#saveStatus"), 'successMsg', 'failMsg');
			$("#saveStatus").text("Failed to Save!");
		})
		.always(function() {
			$("#saveSpinner").hide();
		})
}

function switchClass(el, rem, add) {
	el.removeClass(rem);
	el.addClass(add);
}