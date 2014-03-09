// A few simple javascripts to enable some extra functionality.

(function() {

// Live feedback on the "continue" page
var wordsRemainingElement = document.getElementById('words-remaining');
var wordsRemaining = wordsRemainingElement && parseInt(wordsRemainingElement.innerHTML, 10);

var nextVisible = document.getElementById('next-visible');
var storyWillEnd = document.getElementById('story-will-end');

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

})();
