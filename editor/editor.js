marked.setOptions = {
	gfm: true,
	tables : true,
	sanitize: false
}

var editor = ace.edit("editor");
editor.getSession().setMode("ace/mode/markdown");

var saveButton = $("#saveButton");
var preview = $("#preview");
var previewContainer = $(".previewContainer");

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

function updatePreview() {
	preview.html(marked(editor.getValue()));
	$('pre').each(function(i, block) {
		hljs.highlightBlock(block);
	});
}

function scrollPreview() {
  var ratio = editor.getFirstVisibleRow() / (editor.getSession().getLength() - 1 - (editor.getLastVisibleRow() - editor.getFirstVisibleRow()));
  var scroll = Math.round(ratio * (previewContainer[0].scrollHeight - previewContainer.height()))
  previewContainer.scrollTop(scroll);
}

editor.getSession().on('change', updatePreview);
editor.getSession().on('changeScrollTop', function() {
	setTimeout(scrollPreview, 0);
})