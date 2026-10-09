import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TooltipComponent } from './tooltip.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('TooltipComponent', () => {
  let component: TooltipComponent;
  let fixture: ComponentFixture<TooltipComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ RouterTestingModule, TooltipComponent ]
    })
    .compileComponents();

    fixture = TestBed.createComponent(TooltipComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('keeps the body out of layout as an overlay', () => {
    const body = fixture.nativeElement.querySelector('[data-tooltip-body]') as HTMLElement;
    expect(body.className.split(/\s+/)).toContain('absolute');
  });

  it('expands the inner wrapper when fillHost is enabled', () => {
    component.fillHost = true;
    expect(component.hostClasses.split(/\s+/)).toContain('w-full');
    expect(component.toggleClasses.split(/\s+/)).toContain('text-right');
  });

  it('keeps non-interactive tooltips click-through on hover', () => {
    component.interactive = false;
    const classes = component.classes.split(/\s+/);
    expect(classes).toContain('pointer-events-none');
    expect(classes).not.toContain('group-hover/tooltip:pointer-events-auto');
  });

  it('can stack above dropdown overlays without blocking menus', () => {
    component.stackAboveOverlay = true;
    expect(component.classes.split(/\s+/)).toContain('z-[56]');
  });
});

@Component({
  imports: [TooltipComponent],
  template: `
    <div class="group">
      <button id="row-action">Row action</button>
      <convoy-tooltip [withIcon]="false" [interactive]="false">
        <span tooltipToggle>Status</span><span>Status detail</span>
      </convoy-tooltip>
      <convoy-tooltip [withIcon]="false" [interactive]="false" [fillHost]="true">
        <span tooltipToggle>Rate</span><span>Rate detail</span>
      </convoy-tooltip>
    </div>`
})
class TooltipRowFixture {}

describe('Tooltip focus within a grouped table row', () => {
  it('opens only the focused tooltip and closes it when focus leaves', async () => {
    await TestBed.configureTestingModule({ imports: [TooltipRowFixture] }).compileComponents();
    const fixture = TestBed.createComponent(TooltipRowFixture);
    fixture.detectChanges();
    const rowAction = fixture.nativeElement.querySelector('#row-action') as HTMLButtonElement;
    const triggers = fixture.nativeElement.querySelectorAll('convoy-tooltip button') as NodeListOf<HTMLButtonElement>;
    const bodies = fixture.nativeElement.querySelectorAll('[data-tooltip-body]') as NodeListOf<HTMLElement>;
    const settle = () => new Promise(resolve => setTimeout(resolve, 250));
    rowAction.focus();
    await settle();
    expect(Array.from(bodies, body => getComputedStyle(body).opacity)).toEqual(['0', '0']);
    triggers[0].focus();
    await settle();
    expect(Array.from(bodies, body => getComputedStyle(body).opacity)).toEqual(['1', '0']);
    triggers[1].focus();
    await settle();
    expect(Array.from(bodies, body => getComputedStyle(body).opacity)).toEqual(['0', '1']);
    rowAction.focus();
    await settle();
    expect(Array.from(bodies, body => getComputedStyle(body).opacity)).toEqual(['0', '0']);
  });
});
