import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MultiInputComponent } from './multi-input.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('MultiInputComponent', () => {
  let component: MultiInputComponent;
  let fixture: ComponentFixture<MultiInputComponent>;
  let input: HTMLInputElement;
  let emitted: string[][];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, MultiInputComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(MultiInputComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
    input = fixture.nativeElement.querySelector('input');
    emitted = [];
    component.inputValues.subscribe(values => emitted.push(values));
  });

  const type = (value: string) => {
    input.value = value;
    input.dispatchEvent(new Event('input'));
  };
  const press = (key: string) => {
    const event = new KeyboardEvent('keydown', { key, cancelable: true });
    input.dispatchEvent(event);
    return event;
  };

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('keeps typed text that was never separated when the field loses focus', () => {
    type('request.header.X-GitHub-Delivery');
    input.dispatchEvent(new Event('blur'));

    expect(component.keys).toEqual(['request.header.X-GitHub-Delivery']);
    expect(emitted.at(-1)).toEqual(['request.header.X-GitHub-Delivery']);
    expect(input.value).toBe('');
  });

  it('commits on Enter without submitting the surrounding form', () => {
    type('request.body.id');
    const event = press('Enter');

    expect(event.defaultPrevented).toBeTrue();
    expect(component.keys).toEqual(['request.body.id']);
  });

  it('splits pasted comma-separated values and skips blanks', () => {
    type('a, b,,c');

    expect(component.keys).toEqual(['a', 'b', 'c']);
  });

  it('does not add duplicates', () => {
    type('a,');
    type('a,');

    expect(component.keys).toEqual(['a']);
  });

  it('emits when Backspace removes the last key', () => {
    type('a,b,');
    emitted = [];
    press('Backspace');

    expect(component.keys).toEqual(['a']);
    expect(emitted).toEqual([['a']]);
  });

  it('does not change keys in view mode', () => {
    component.keys = ['a'];
    component.action = 'view';
    press('Backspace');

    expect(component.keys).toEqual(['a']);
  });

  it('gives each instance its own input id for the label', () => {
    const other = TestBed.createComponent(MultiInputComponent).componentInstance;

    expect(other.inputId).not.toBe(component.inputId);
  });
});
