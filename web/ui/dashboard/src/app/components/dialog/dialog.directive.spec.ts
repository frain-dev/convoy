import { ElementRef } from '@angular/core';
import { DialogDirective } from './dialog.directive';

describe('DialogDirective', () => {
  it('should create an instance', () => {
    const directive = new DialogDirective({ nativeElement: document.createElement('dialog') } as ElementRef<HTMLDialogElement>);
    expect(directive).toBeTruthy();
  });
});
