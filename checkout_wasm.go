package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"syscall/js"
)

// set client pk on compile
var stripePK string

type item struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
	Qty    int    `json:"quantity"`
}

var (
	doc        = js.Global().Get("document")
	body       = doc.Call("querySelector", "body")
	bodystring = body.Get("innerHTML").String()
	cart       []item
)

func main() {
	c := make(chan struct{}, 0)
	if stripePK == "" {
		log.Fatal("Stripe PK not found!")
	}
	window := js.Global().Get("window")
	location := window.Get("location")
	pathname := location.Get("pathname").String()
	log.Printf("Current Pathname: %s", pathname)

	switch pathname {
	case "/":
		defaultLogic()
	case "/complete":
		completeLogic()
	default:
		log.Printf("Unknown Pathname: %s", pathname)
	}
	<-c
}

func defaultLogic() {
	js.Global().Set("addToCart", js.FuncOf(addUnToCart))
	js.Global().Set("clearStorage", js.FuncOf(clearAll))
	js.Global().Set("emptyCart", js.FuncOf(emptyCart))
	js.Global().Set("updateItemQuantity", js.FuncOf(updateItemQuantity))
	js.Global().Set("removeFromCart", js.FuncOf(removeFromCart))
	js.Global().Set("addShippingInfo", js.FuncOf(addShippingInfo))
	js.Global().Set("goToCheckout", js.FuncOf(goToCheckout))
	js.Global().Set("cancelCheckout", js.FuncOf(cancelCheckout))

	loadCart()
	updateCartDisplay()
}

func saveCart() {
	cartJSON, err := json.Marshal(cart)
	if err != nil {
		log.Println("Error saving cart:", err)
		return
	}
	js.Global().Get("localStorage").Call("setItem", "cartItems", string(cartJSON))
	updateCartDisplay()
}

func addToCart(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return "Error: Missing arguments"
	}
	var cartItem item
	index := -1
	id := args[0].String()
	qty := args[2].Int()
	if qty == 0 {
		qty = 1
	}
	amount := int(args[1].Float()) * qty
	for i, _ := range cart {
		if strings.Split(cart[i].ID, "|")[0] == strings.Split(id, "|")[0] {
			index = i
		}
	}
	if index > -1 {
		// update shipping
		if strings.Split(cart[index].ID, "|")[0] == "shipping-to" {
			cart[index].ID = id
			cart[index].Qty = 1
			cart[index].Amount = amount
		} else {
			cart[index].Qty = cart[index].Qty + qty
			cart[index].Amount = cart[index].Amount + amount
		}
	} else {
		cartItem = item{
			ID:     id,
			Amount: amount,
			Qty:    qty,
		}
		cart = append(cart, cartItem)
	}
	saveCart()
	return nil
}

func addUnToCart(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return "Error: Missing arguments"
	}
	id := args[0].String()
	price := args[1].Float()
	quantityInput := doc.Call("getElementById", fmt.Sprintf("qty-%s", id))
	if !quantityInput.Truthy() {
		println("Error: Quantity input not found for item", id)
		return nil
	}
	quantity, err := strconv.Atoi(quantityInput.Get("value").String())
	if err != nil || quantity < 1 {
		quantity = 1
	}

	addToCart(js.Value{}, []js.Value{
		js.ValueOf(id),
		js.ValueOf(int(price * 100)),
		js.ValueOf(quantity),
	})
	return nil
}

func removeFromCart(this js.Value, inputs []js.Value) interface{} {
	id := inputs[0].String()
	newCart := []item{}
	for _, m := range cart {
		if m.ID != id {
			newCart = append(newCart, m)
		}
	}
	cart = newCart
	saveCart()
	return nil
}

func loadCart() {
	storedCart := js.Global().Get("localStorage").Call("getItem", "cartItems")
	if !storedCart.IsUndefined() && !storedCart.IsNull() {
		err := json.Unmarshal([]byte(storedCart.String()), &cart)
		if err != nil {
			log.Println(`can't unmarshal cart from local storage`)
			cart = []item{}
		}
	}
}

func emptyCart(this js.Value, inputs []js.Value) interface{} {
	js.Global().Get("localStorage").Call("removeItem", "cartItems")
	cart = []item{}
	updateCartDisplay()
	return nil
}

func clearAll(this js.Value, inputs []js.Value) interface{} {
	js.Global().Get("localStorage").Call("clear")
	cart = []item{}
	updateCartDisplay()
	return nil
}

func updateCartDisplay() {
	cartContainer := doc.Call("getElementById", "cart-items")
	totalPriceElement := doc.Call("getElementById", "total-price")
	table := cartContainer.Call("querySelector", "table")
	if table.IsNull() {
		table = doc.Call("createElement", "table")
		thead := doc.Call("createElement", "thead")
		thead.Set("innerHTML", `<tr><th>Item</th><th>Price</th><th>Quantity</th><th>Actions</th></tr>`)
		table.Call("appendChild", thead)
		tbody := doc.Call("createElement", "tbody")
		tbody.Set("id", "cart-tbody")
		table.Call("appendChild", tbody)
		cartContainer.Call("appendChild", table)
	}
	tbody := doc.Call("getElementById", "cart-tbody")
	tbody.Set("innerHTML", "")

	total := 0
	hasShipping := false
	for _, m := range cart {
		total += m.Amount
		row := doc.Call("createElement", "tr")

		row.Set("innerHTML", fmt.Sprintf(`<td>%s</td><td>$%.2f</td><td>%s</td><td><button onclick='removeFromCart("%s")'>Remove</button></td>`,
			func() string {
				parts := strings.Split(m.ID, "|")
				if len(parts) < 8 {
					return m.ID
				}
				hasShipping = true
				return fmt.Sprintf("%s:<br>%s<br>%s<br>%s, %s %s<br>%s<br>%s", parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6], parts[7])
			}(),
			float64(m.Amount)/100,
			func() string {
				if len(strings.Split(m.ID, "|")) == 8 {
					return ""
				}
				return fmt.Sprintf(`<input type='number' value='%d' min='1' onchange='updateItemQuantity("%s", this.value)'>`, m.Qty, m.ID)
			}(),
			m.ID,
		))
		tbody.Call("appendChild", row)
	}
	totalPriceElement.Set("textContent", fmt.Sprintf("Total: $%.2f", float64(total)/100))

	checkoutbutton := doc.Call("getElementById", "checkout-button")
	if !checkoutbutton.Truthy() {
		return
	}

	if len(cart) > 1 && hasShipping {
		checkoutbutton.Call("removeAttribute", "disabled")
	} else {
		checkoutbutton.Call("setAttribute", "disabled", "true")
	}
}

func updateItemQuantity(this js.Value, args []js.Value) interface{} {
	id := args[0].String()
	qty, err := strconv.Atoi(args[1].String())
	if err != nil {
		log.Println(err)
	}
	for i, _ := range cart {
		if cart[i].ID == id {
			unitPrice := cart[i].Amount / cart[i].Qty
			cart[i].Qty = qty
			cart[i].Amount = unitPrice * qty
			break
		}
	}
	saveCart()
	return nil
}

func addShippingInfo(this js.Value, args []js.Value) interface{} {
	event := args[0]
	form := args[1]
	event.Call("preventDefault")
	getFormValue := func(name string) string {
		return form.Call("querySelector", fmt.Sprintf("[name='%s']", name)).Get("value").String()
	}
	shippingInfo := fmt.Sprintf("shipping-to|%s|%s|%s|%s|%s|%s|%s",
		getFormValue("shipping-name"),
		getFormValue("shipping-address"),
		getFormValue("shipping-city"),
		getFormValue("shipping-state"),
		getFormValue("shipping-zip"),
		getFormValue("shipping-country"),
		getFormValue("shipping-phone"),
	)
	priceStr := getFormValue("shipping-price")
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		println("Error: Failed to parse shipping price")
		price = 0.0
	}

	addToCart(js.Value{}, []js.Value{
		js.ValueOf(shippingInfo),
		js.ValueOf(int(price * 100)),
		js.ValueOf(1),
	})
	return false
}

var (
	elements       js.Value
	stripeValue    js.Value
	stripe         js.Value
	checkoutButton = doc.Call("getElementById", "checkout-button")
	checkoutDiv    = doc.Call("getElementById", "checkout-container")
	checkoutStripe = doc.Call("getElementById", "stripecheckout")
)

// Save the original innerHTML of the parent
func goToCheckout(this js.Value, args []js.Value) any {
	if stripeValue.IsUndefined() {
		log.Println(`js.Global().Get("Stripe")`)
		stripeValue = js.Global().Get("Stripe")
		if stripeValue.IsUndefined() {
			log.Println(`Stripe is undefined`)
			return nil
		}
	}
	log.Println("Stripe.js loaded successfully")

	if stripe.IsUndefined() {
		log.Println("Invoking Stripe")
		//	stripe = stripeValue.Invoke("pk_test...")
		stripe = stripeValue.Invoke(stripePK)
		if stripe.IsUndefined() {
			log.Println("Failed to invoke Stripe")
			return nil
		}
	}
	log.Println("Stripe initialized")
	checkoutStripe.Call("showModal")
	log.Println("initializePayment()")
	initializePayment()
	return nil
}

func cancelCheckout(this js.Value, args []js.Value) any {
	log.Println("Cancelling checkout ; closing dialog")
	checkoutStripe.Call("close")
	updateCartDisplay()
	return nil
}

func initializePayment() {
	type cItem struct {
		ID     string `json:"id"`
		Amount int    `json:"amount"`
	}
	type checkout struct {
		Items []cItem `json:"items"`
	}
	payload := checkout{
		Items: func() []cItem {
			var items []cItem
			for _, it := range cart {
				items = append(items, cItem{ID: it.ID + " X " + strconv.Itoa(it.Qty), Amount: it.Amount})
			}
			return items
		}(),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshaling JSON:", err)
		return
	}
	fetchInit := map[string]interface{}{
		"method": "POST",
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
		},
		"body": string(payloadJSON),
	}

	log.Println("fetch  /create-payment-intent")
	js.Global().Call("fetch", "/create-payment-intent", js.ValueOf(fetchInit)).
		Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			response := args[0]
			log.Println("got response from fetch /create-payment-intent")
			if !response.Get("ok").Bool() {
				log.Println("Fetch request failed with status:", response.Get("status").Int())
				showMessage("Failed to create payment intent: " + response.Get("status").String())
				return nil
			}
			response.Call("json").Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				clientSecret := args[0].Get("clientSecret").String()
				log.Println("Client secret received:", clientSecret)
				setupStripeElements(clientSecret)
				return nil
			})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				log.Println("Error parsing JSON response:", args[0])
				showMessage("Failed to parse payment intent response.")
				return nil
			}))
			return nil
		})).
		Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			log.Println("Error in fetch request:", args[0])
			showMessage("Failed to communicate with the server.")
			return nil
		}))
}

func setupStripeElements(clientSecret string) {
	elements = stripe.Call("elements", map[string]interface{}{
		"clientSecret": clientSecret,
	})
	if elements.IsUndefined() {
		log.Println("Failed to initialize Stripe Elements")
		showMessage("Failed to initialize payment elements.")
		return
	}
	paymentElement := elements.Call("create", "payment", map[string]interface{}{
		"layout": "tabs",
	})
	if paymentElement.IsUndefined() {
		log.Println("Failed to create payment element")
		showMessage("Failed to create payment element.")
		return
	}
	paymentElement.Call("mount", "#payment-element")
	submitButton := doc.Call("getElementById", "submit")
	submitButton.Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		showSpinner(true)
		confirmPayment(clientSecret)
		return nil
	}))
}

func confirmPayment(clientSecret string) {

	windowLocation := js.Global().Get("window").Get("location")
	protocol := windowLocation.Get("protocol").String()
	hostname := windowLocation.Get("hostname").String()
	port := windowLocation.Get("port").String()

	baseURL := protocol + "//" + hostname
	if port != "" {
		baseURL += ":" + port
	}
	//	path := windowLocation.Get("pathname").String()
	//    baseURL += strings.Split(path, "?")[0]
	//    log.Println("return url ", baseURL)

	returnURL := baseURL + "/complete"
	returnURL += "?payment_intent=" + clientSecret // + "#complete"
	log.Println("Return URL for payment:", returnURL)

	stripe.Call("confirmPayment", map[string]interface{}{
		"elements": elements,
		"confirmParams": map[string]interface{}{
			"return_url": returnURL,
		},
	}).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		result := args[0]
		if result.Get("error").IsUndefined() {
			log.Println("Payment successful:", result)
			showMessage("Payment successful! Thank you for your order.")
		} else {
			log.Println("Payment error:", result.Get("error").Get("message").String())
			showMessage("Payment failed: " + result.Get("error").Get("message").String())
		}

		showSpinner(false)
		return nil
	}))
}

func showMessage(message string) {
	messageElement := doc.Call("getElementById", "payment-message")
	messageElement.Set("innerText", message)
	messageElement.Set("className", "")
}

func showSpinner(isLoading bool) {
	spinner := doc.Call("getElementById", "spinner")
	buttonText := doc.Call("getElementById", "button-text")

	if isLoading {
		spinner.Set("className", "")
		buttonText.Set("className", "hidden")
	} else {
		spinner.Set("className", "hidden")
		buttonText.Set("className", "")
	}
}

// /complete

func completeLogic() {
	initializeStripe()
}

func initializeStripe() {
	stripeValue := js.Global().Get("Stripe")
	if stripeValue.IsUndefined() {
		log.Println("Stripe is not defined")
		return
	}

	// stripe = stripeValue.Invoke("pk_...")
	stripe = stripeValue.Invoke(stripePK)

	if stripe.IsUndefined() {
		log.Println("Failed to initialize Stripe")
		return
	}
	checkStatus()
}

var (
	successIcon = `<svg width="16" height="14" viewBox="0 0 16 14" fill="none" xmlns="http://www.w3.org/2000/svg">
		<path fill-rule="evenodd" clip-rule="evenodd" d="M15.4695 0.232963C15.8241 0.561287 15.8454 1.1149 15.5171 1.46949L6.14206 11.5945C5.97228 11.7778 5.73221 11.8799 5.48237 11.8748C5.23253 11.8698 4.99677 11.7582 4.83452 11.5681L0.459523 6.44311C0.145767 6.07557 0.18937 5.52327 0.556912 5.20951C0.924454 4.89575 1.47676 4.93936 1.79051 5.3069L5.52658 9.68343L14.233 0.280522C14.5613 -0.0740672 15.1149 -0.0953599 15.4695 0.232963Z" fill="white"/>
	</svg>`

	errorIcon = `<svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
		<path fill-rule="evenodd" clip-rule="evenodd" d="M1.25628 1.25628C1.59799 0.914573 2.15201 0.914573 2.49372 1.25628L8 6.76256L13.5063 1.25628C13.848 0.914573 14.402 0.914573 14.7437 1.25628C15.0854 1.59799 15.0854 2.15201 14.7437 2.49372L9.23744 8L14.7437 13.5063C15.0854 13.848 15.0854 14.402 14.7437 14.7437C14.402 15.0854 13.848 15.0854 13.5063 14.7437L8 9.23744L2.49372 14.7437C2.15201 15.0854 1.59799 15.0854 1.25628 14.7437C0.914573 14.402 0.914573 13.848 1.25628 13.5063L6.76256 8L1.25628 2.49372C0.914573 2.15201 0.914573 1.59799 1.25628 1.25628Z" fill="white"/>
	</svg>`

	infoIcon = `<svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
		<path fill-rule="evenodd" clip-rule="evenodd" d="M10 1.5H4C2.61929 1.5 1.5 2.61929 1.5 4V10C1.5 11.3807 2.61929 12.5 4 12.5H10C11.3807 12.5 12.5 11.3807 12.5 10V4C12.5 2.61929 11.3807 1.5 10 1.5ZM4 0C1.79086 0 0 1.79086 0 4V10C0 12.2091 1.79086 14 4 14H10C12.2091 14 14 12.2091 14 10V4C14 1.79086 12.2091 0 10 0H4Z" fill="white"/>
		<path fill-rule="evenodd" clip-rule="evenodd" d="M5.25 7C5.25 6.58579 5.58579 6.25 6 6.25H7.25C7.66421 6.25 8 6.58579 8 7V10.5C8 10.9142 7.66421 11.25 7.25 11.25C6.83579 11.25 6.5 10.9142 6.5 10.5V7.75H6C5.58579 7.75 5.25 7.41421 5.25 7Z" fill="white"/>
		<path d="M5.75 4C5.75 3.31075 6.31075 2.75 7 2.75C7.68925 2.75 8.25 3.31075 8.25 4C8.25 4.68925 7.68925 5.25 7 5.25C6.31075 5.25 5.75 4.68925 5.75 4Z" fill="white"/>
	</svg>`
)

func setErrorState() {
	js.Global().Get("document").Call("querySelector", "#status-icon").Set("style", map[string]interface{}{"backgroundColor": "#DF1B41"})
	js.Global().Get("document").Call("querySelector", "#status-icon").Set("innerHTML", errorIcon)
	js.Global().Get("document").Call("querySelector", "#status-text").Set("textContent", "Something went wrong, please try again.")
	js.Global().Get("document").Call("querySelector", "#details-table").Call("classList").Call("add", "hidden")
	js.Global().Get("document").Call("querySelector", "#view-details").Call("classList").Call("add", "hidden")
}

func checkStatus() {
	clientSecret := js.Global().Get("URLSearchParams").New(js.Global().Get("window").Get("location").Get("search")).Call("get", "payment_intent_client_secret").String()

	if clientSecret == "" {
		setErrorState()
		return
	}

	if stripe.IsUndefined() {
		log.Println("Stripe is not initialized")
		setErrorState()
		return
	}

	stripe.Call("retrievePaymentIntent", clientSecret).Call("then", js.FuncOf(func(this js.Value, p []js.Value) interface{} {
		paymentIntent := p[0].Get("paymentIntent")
		setPaymentDetails(paymentIntent)
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, p []js.Value) interface{} {
		setErrorState()
		return nil
	}))
}

func getAllLocalStorageData() map[string]interface{} {
	localStorage := js.Global().Get("localStorage")
	keys := js.Global().Get("Object").Call("keys", localStorage)
	data := make(map[string]interface{})

	for i := 0; i < keys.Length(); i++ {
		key := keys.Index(i).String()
		value := localStorage.Call("getItem", key).String()
		var parsedValue interface{}
		err := json.Unmarshal([]byte(value), &parsedValue)
		if err != nil {
			parsedValue = value // If not JSON, store raw value
		}
		data[key] = parsedValue
	}
	return data
}

func submitOrder(localStorageData map[string]interface{}, paymentIntentId string) {
	orderData := map[string]interface{}{
		"localStorageData": localStorageData,
		"paymentIntentId":  paymentIntentId,
	}

	body, err := json.Marshal(orderData)
	if err != nil {
		log.Println("Error marshalling order data:", err)
		return
	}

	fetch := js.Global().Get("fetch")
	if fetch.IsUndefined() {
		log.Println("Fetch API is not available")
		return
	}

	options := map[string]interface{}{
		"method": "POST",
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
		},
		"body": string(body),
	}

	fetch.Invoke("/submit-order", js.ValueOf(options)).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		response := args[0]
		response.Call("json").Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			data := args[0]
			log.Println("Order submitted successfully:", data)
			return nil
		})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			err := args[0]
			log.Println("Error parsing order response:", err)
			return nil
		}))
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		err := args[0]
		log.Println("Error submitting order:", err)
		return nil
	}))
}

func setPaymentDetails(intent js.Value) {
	var statusText, iconColor, icon string
	statusText = "Something went wrong, please try again."
	iconColor = "#DF1B41"
	icon = errorIcon

	if !intent.IsUndefined() {
		intentStatus := intent.Get("status").String()
		intentID := intent.Get("id").String()

		allLocalStorageData := getAllLocalStorageData()

		switch intentStatus {
		case "succeeded":
			statusText = "Payment succeeded"
			iconColor = "#30B130"
			icon = successIcon
			if len(allLocalStorageData) > 0 {
				submitOrder(allLocalStorageData, intentID)
			} else {
				log.Println("No data found in localStorage; order not submitted.")
			}
		case "processing":
			statusText = "Your payment is processing."
			iconColor = "#6D6E78"
			icon = infoIcon
			if len(allLocalStorageData) > 0 {
				submitOrder(allLocalStorageData, intentID)
			} else {
				log.Println("No data found in localStorage; order not submitted.")
			}
		case "requires_payment_method":
			statusText = "Your payment was not successful, please try again."
		default:
			statusText = "Unknown payment status."
		}

		// Update the status icon, text, and links
		js.Global().Get("document").Call("querySelector", "#status-icon").Set("style", map[string]interface{}{"backgroundColor": iconColor})
		js.Global().Get("document").Call("querySelector", "#status-icon").Set("innerHTML", icon)
		js.Global().Get("document").Call("querySelector", "#status-text").Set("textContent", statusText)
		js.Global().Get("document").Call("querySelector", "#intent-id").Set("textContent", intentID)
		js.Global().Get("document").Call("querySelector", "#intent-status").Set("textContent", intentStatus)
		js.Global().Get("document").Call("querySelector", "#view-details").Set("href", "https://dashboard.stripe.com/payments/"+intentID)

		// Update the "Order Details" link with the paymentIntent ID
		orderDetailsLink := js.Global().Get("document").Call("querySelector", "#order-details-link")
		orderDetailsLink.Set("href", "/order/"+intentID)
		orderDetailsLink.Set("onclick", nil) // Allow default behavior (navigation)

	} else {
		setErrorState()
	}
}
