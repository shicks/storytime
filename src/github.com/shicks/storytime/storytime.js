// A few simple javascripts to enable some extra functionality.

(function() {

// Live feedback on the "continue" page
var wordsRemainingElement = document.getElementById('words-remaining');
var wordsRemaining = wordsRemainingElement && parseInt(wordsRemainingElement.innerHTML, 10);

var nextVisible = document.getElementById('next-visible');
var storyWillEnd = document.getElementById('story-will-end');
var submitButton = document.getElementById('submit');

var continueText = document.getElementById('continue-text');
if (continueText) continueText.addEventListener('keyup', update);

function show(elt, enable) {
  if (!elt) return;
  if (enable) elt.classList.remove('invisible');
  if (!enable) elt.classList.add('invisible');
}

function update() {
  // Update words-remaining
  var left = wordsRemaining - countWords(continueText.value);
  show(storyWillEnd, left <= 0);
  show(wordsRemainingElement, left > 0);
  wordsRemainingElement.textContent = left + '';

  // Update next-visible
  if (nextVisible) {
    nextVisible.textContent = lastWords(continueText.value, 16); // TODO(sdh): inject count
  }

  // Disable the submit button if story is too long
  submitButton.disabled = continueText.value.length > 500;
}

function countWords(text) {
  var words = text.trim().split(/\s+/);
  return words[0] ? words.length : 0;
}

function lastWords(text, count) {
  var lines = text.split(/\n/);
  for (var i = lines.length - 1; i >= 0; i--) {
    if (lines[i].trim()) {
      text = lines[i];
      break;
    }
  }
  var words = text.trim().split(/\s+/);
  if (words.length > count) {
    words = words.slice(words.length - count);
  }
  return words.join(' ');
}

// Prettify the "story" page

var colors = ['#000000', '#98224a', '#519049', '#144566', '#2c777e',
              '#844e2c', '#cf7208', '#b10038', '#45483f', '#8f8387'];
var authorCount = 0;
var authors = {};
var parts = document.getElementsByClassName('story-part');
for (var i = 0; i < parts.length; i++) {
  var part = parts[i];
  var author = part.dataset['author'];
  if (!(author in authors)) {
    authors[author] = colors[authorCount++ % colors.length];
  }
  part.style.color = authors[author];
  part.title = 'Written by ' + author; // TODO(sdh): date?
}

})();
