import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Directive, ElementRef, EventEmitter, Host, Input, OnInit, Output, ViewChild } from '@angular/core';
import { ButtonComponent } from '../button/button.component';
import { DropdownContainerComponent } from '../dropdown-container/dropdown-container.component';
import { OverlayDirective } from '../overlay/overlay.directive';

@Directive({
	selector: '[convoy-dropdown-option], [convoy-dropdown-close]',
	standalone: true,
	host: { '(click)': 'onSelectOption()' }
})
export class DropdownOptionDirective {
    @Output() readonly onSelect = new EventEmitter<any>;
    @Input('value') value: any = undefined;
    parent: DropdownComponent;

    constructor(@Host() parent: DropdownComponent) {
        this.parent = parent;

    }

    onSelectOption() {
        this.onSelect.emit();
        this.parent.show = false;
        this.parent.onSelect.emit(this.value);
    }
}

@Component({
	selector: 'convoy-dropdown, [convoy-dropdown]',
	standalone: true,
	imports: [CommonModule, ButtonComponent, DropdownContainerComponent, OverlayDirective, DropdownOptionDirective],
	templateUrl: './dropdown.component.html',
    styleUrls: ['./dropdown.component.scss'],
    changeDetection: ChangeDetectionStrategy.Default
})
export class DropdownComponent implements OnInit {
	@Input('position') position: 'right' | 'left' | 'center' | 'right-side' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' | 'xl' | 'full' = 'md';
	@Input('show') showDropdown = false;
	@ViewChild('dropdownTriggerContainer', { static: true }) dropdownTriggerContainer!: ElementRef;
	@ViewChild('dropdownContainer', { static: true }) dropdownOptions!: ElementRef;
    @Output() readonly onSelect = new EventEmitter<any>;
	show = false;

    constructor() {}

    ngOnInit(): void {
        if(this.showDropdown) this.show = true;

		this.dropdownTriggerContainer.nativeElement.children[0].addEventListener('click', () => (this.show = !this.show));
    }
}
